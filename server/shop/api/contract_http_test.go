package api_test

// 说明：接口契约（api-contract.md）全链路 HTTP 测试，挂在 api 包（外部测试包）内，
// 使 api 层「原型处理器 + 契约兼容层 + 员工分区」在 api 包覆盖统计中真实计数。
// 与 router 包的集成测试互补：router 关注路由装配与门禁，这里关注处理器行为覆盖率。
// 使用真实 MySQL（shop_test，须 -p 1 串行）。

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/config"
	shopdb "github.com/flipped-aurora/gin-vue-admin/server/shop/db"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/migrations"
	shoprouter "github.com/flipped-aurora/gin-vue-admin/server/shop/router"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/service"
)

var (
	extOnce sync.Once
	extDB   *gorm.DB
	extCfg  config.Config
	extErr  error
)

func extSetup(t *testing.T) (*gin.Engine, *service.SimulatedChannel) {
	t.Helper()
	extOnce.Do(func() {
		dsn := os.Getenv("SHOP_TEST_DSN")
		if dsn == "" {
			dsn = "root:root@tcp(127.0.0.1:3307)/shop_test?charset=utf8mb4&parseTime=True&loc=Local"
		}
		extCfg = config.Default()
		extCfg.Mysql.DSN = dsn
		extCfg.Shop.PayNotifySecret = "test-secret"
		extCfg.Shop.OrderTimeoutMinutes = 10
		// 演示模式开启：账号列表、强制过期、演示重置等演示钩子才可用
		extCfg.Shop.DemoMode = true
		db, err := shopdb.Gorm(extCfg)
		if err != nil {
			extErr = err
			return
		}
		if err := shopdb.Migrate(db, migrations.FS); err != nil {
			extErr = err
			return
		}
		if err := shopdb.SeedAccounts(db, "shop123"); err != nil {
			extErr = err
			return
		}
		extDB = db
	})
	if extErr != nil {
		t.Fatalf("初始化测试库失败（需 MySQL 3307 可用）: %v", extErr)
	}

	tables := []string{
		"sessions",
		"abnormal_payments", "payments", "payment_notices", "refunds", "shipments",
		"inventory_ledger", "operation_logs", "orders", "products", "users",
	}
	for _, tbl := range tables {
		if err := extDB.Exec("DELETE FROM " + tbl).Error; err != nil {
			t.Fatalf("清理 %s 失败: %v", tbl, err)
		}
	}
	if err := shopdb.Migrate(extDB, migrations.FS); err != nil {
		t.Fatalf("重放迁移失败: %v", err)
	}
	if err := shopdb.SeedAccounts(extDB, "shop123"); err != nil {
		t.Fatalf("重放种子账号失败: %v", err)
	}

	gin.SetMode(gin.TestMode)
	cfg := extCfg
	ch := service.NewSimulatedChannel(true)
	g := service.NewServiceGroup(service.Deps{DB: extDB, Now: time.Now, Cfg: cfg, Channel: ch})
	r := gin.New()
	shoprouter.InitShopRouter(r.Group(""), g, cfg, time.Now)
	return r, ch
}

func extDo(t *testing.T, r *gin.Engine, method, path, body, token string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	return rr
}

func extStatus(t *testing.T, rr *httptest.ResponseRecorder, want int, ctx string) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("%s: 期望 %d 实际 %d body=%s", ctx, want, rr.Code, rr.Body.String())
	}
}

func extJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是 JSON: %v body=%s", err, rr.Body.String())
	}
	return out
}

func extLogin(t *testing.T, r *gin.Engine, username string) string {
	t.Helper()
	rr := extDo(t, r, "POST", "/api/login", `{"username":"`+username+`","password":"shop123"}`, "", nil)
	extStatus(t, rr, http.StatusOK, "登录 "+username)
	token, _ := extJSON(t, rr)["token"].(string)
	if token == "" {
		t.Fatalf("登录 %s 未返回 token: %s", username, rr.Body.String())
	}
	return token
}

// extSign 契约通道签名：对原始请求体做 HMAC-SHA256（X-Channel-Signature）。
func extSign(secret string, raw []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

func jsonNum(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// 走契约通道创建并支付一笔订单，返回订单号与金额。
func extPaidOrder(t *testing.T, r *gin.Engine, buyerToken, txn string) (string, int64) {
	t.Helper()
	rr := extDo(t, r, "POST", "/api/orders", `{"product_id":"SKU-A","quantity":1}`, buyerToken, nil)
	extStatus(t, rr, http.StatusOK, "下单")
	res := extJSON(t, rr)
	orderNo, _ := res["order_no"].(string)
	amount := int64(0)
	if f, ok := res["amount"].(float64); ok {
		amount = int64(f)
	}
	if orderNo == "" || amount <= 0 {
		t.Fatalf("下单响应缺少字段: %s", rr.Body.String())
	}
	body := `{"order_no":"` + orderNo + `","channel_txn_no":"` + txn + `","amount":` + jsonNum(amount) + `}`
	rr = extDo(t, r, "POST", "/api/pay/notify", body, "", map[string]string{
		"X-Channel-Signature": extSign(extCfg.Shop.PayNotifySecret, []byte(body)),
	})
	extStatus(t, rr, http.StatusOK, "支付通知")
	return orderNo, amount
}

// TestContractStaffFlows 覆盖运营 / 仓库 / 客服三个分区的契约接口与越权门禁。
func TestContractStaffFlows(t *testing.T) {
	r, _ := extSetup(t)

	buyer := extLogin(t, r, "buyer1")
	ops := extLogin(t, r, "ops1")
	wh := extLogin(t, r, "wh1")
	cs := extLogin(t, r, "cs1")

	// 公开区：商品、账号列表、健康检查、买家端页面
	extStatus(t, extDo(t, r, "GET", "/api/products", "", "", nil), http.StatusOK, "商品列表")
	extStatus(t, extDo(t, r, "GET", "/api/users", "", "", nil), http.StatusOK, "账号列表")
	extStatus(t, extDo(t, r, "GET", "/api/health", "", "", nil), http.StatusOK, "健康检查")
	extStatus(t, extDo(t, r, "GET", "/shop", "", "", nil), http.StatusOK, "买家端页面")

	// 未登录访问分区接口 -> 401
	extStatus(t, extDo(t, r, "GET", "/api/ops/summary", "", "", nil), http.StatusUnauthorized, "未登录运营接口")

	orderNo, amount := extPaidOrder(t, r, buyer, "TXN-API-1")

	// 买家区：我的订单 / 详情 / 退款申请（未发货前可申请）
	extStatus(t, extDo(t, r, "GET", "/api/orders", "", buyer, nil), http.StatusOK, "我的订单")
	extStatus(t, extDo(t, r, "GET", "/api/orders/"+orderNo, "", buyer, nil), http.StatusOK, "订单详情（买家）")
	extStatus(t, extDo(t, r, "GET", "/api/me", "", buyer, nil), http.StatusOK, "当前登录信息")
	// 买家越权访问运营接口 -> 403
	extStatus(t, extDo(t, r, "GET", "/api/ops/summary", "", buyer, nil), http.StatusForbidden, "买家访问运营接口")

	// 运营区：汇总 / 订单 / 核查 / 通知 / 异常 / 流水 / 留痕 / 支付记录 / 商品维护
	extStatus(t, extDo(t, r, "GET", "/api/ops/summary", "", ops, nil), http.StatusOK, "运营汇总")
	extStatus(t, extDo(t, r, "GET", "/api/ops/orders?status=paid", "", ops, nil), http.StatusOK, "运营订单列表")
	extStatus(t, extDo(t, r, "GET", "/api/ops/orders/"+orderNo, "", ops, nil), http.StatusOK, "运营订单核查")
	extStatus(t, extDo(t, r, "GET", "/api/ops/notices?orderNo="+orderNo, "", ops, nil), http.StatusOK, "通知记录（按单）")
	extStatus(t, extDo(t, r, "GET", "/api/ops/notices?limit=10", "", ops, nil), http.StatusOK, "通知记录（按结果）")
	extStatus(t, extDo(t, r, "GET", "/api/ops/abnormals", "", ops, nil), http.StatusOK, "异常收款")
	extStatus(t, extDo(t, r, "GET", "/api/ops/ledger?productId=1", "", ops, nil), http.StatusOK, "库存流水")
	extStatus(t, extDo(t, r, "GET", "/api/ops/trail?orderNo="+orderNo, "", ops, nil), http.StatusOK, "操作留痕")
	extStatus(t, extDo(t, r, "GET", "/api/ops/payments?orderNo="+orderNo, "", ops, nil), http.StatusOK, "支付记录")
	// 原型商品维护按数字主键寻址：契约视图用 db_id 暴露真实主键（种子自增 ID 不固定）
	products := extJSON(t, extDo(t, r, "GET", "/api/products", "", "", nil))["data"].([]interface{})
	firstProduct := products[0].(map[string]interface{})
	pid := uint64(firstProduct["db_id"].(float64))
	extStatus(t, extDo(t, r, "PATCH", "/api/ops/products", `{"productId":`+jsonNum(int64(pid))+`,"priceCents":10000}`, ops, nil), http.StatusOK, "商品调价")
	extStatus(t, extDo(t, r, "PATCH", "/api/ops/products/SKU-A", `{"available":5}`, ops, nil), http.StatusOK, "覆盖可售")
	extStatus(t, extDo(t, r, "PATCH", "/api/ops/products/SKU-A", `{"price_cents":10000,"on_sale":true}`, ops, nil), http.StatusOK, "按 SKU 调价")
	extStatus(t, extDo(t, r, "PATCH", "/api/ops/products/SKU-A", `{}`, ops, nil), http.StatusBadRequest, "空入参")

	// 仓库区：待发货 / 发货 / 重复发货 / 记录 / 详情
	extStatus(t, extDo(t, r, "GET", "/api/warehouse/shipable", "", wh, nil), http.StatusOK, "待发货列表")
	extStatus(t, extDo(t, r, "GET", "/api/warehouse/orders", "", wh, nil), http.StatusOK, "待发货列表（契约）")
	extStatus(t, extDo(t, r, "POST", "/api/warehouse/ship", `{"orderNo":"`+orderNo+`","carrier":"顺丰","trackingNo":"SF-1"}`, wh, nil), http.StatusOK, "发货登记")
	extStatus(t, extDo(t, r, "POST", "/api/warehouse/ship", `{"orderNo":"`+orderNo+`","carrier":"顺丰","trackingNo":"SF-2"}`, wh, nil), http.StatusConflict, "重复发货")
	extStatus(t, extDo(t, r, "GET", "/api/warehouse/shipments", "", wh, nil), http.StatusOK, "发货记录")
	extStatus(t, extDo(t, r, "GET", "/api/warehouse/orders/"+orderNo, "", wh, nil), http.StatusOK, "订单详情（仓库）")
	// 仓库越权访问客服接口 -> 403
	extStatus(t, extDo(t, r, "GET", "/api/cs/refunds", "", wh, nil), http.StatusForbidden, "仓库访问客服接口")

	// 客服区：退款列表 / 审核（售前拒绝）/ 退款重试 / 异常收款 / 通知 / 订单详情
	orderNo2, _ := extPaidOrder(t, r, buyer, "TXN-API-2")
	rr := extDo(t, r, "POST", "/api/orders/"+orderNo2+"/refund", `{"reason":"不想要了"}`, buyer, nil)
	extStatus(t, rr, http.StatusOK, "退款申请")
	refundID := ""
	if refund, ok := extJSON(t, rr)["refund"].(map[string]interface{}); ok {
		refundID, _ = refund["refund_no"].(string)
	}

	extStatus(t, extDo(t, r, "GET", "/api/cs/refunds", "", cs, nil), http.StatusOK, "退款单列表")
	extStatus(t, extDo(t, r, "POST", "/api/cs/refunds/audit", `{"orderNo":"`+orderNo2+`","approve":false,"reason":"即将发货"}`, cs, nil), http.StatusOK, "退款审核-拒绝")
	if refundID != "" {
		extStatus(t, extDo(t, r, "POST", "/api/cs/refunds/"+refundID+"/retry", `{}`, cs, nil), http.StatusConflict, "退款重试-非失败态")
	}
	extStatus(t, extDo(t, r, "GET", "/api/cs/payment-exceptions?status=pending", "", cs, nil), http.StatusOK, "异常支付列表")
	extStatus(t, extDo(t, r, "GET", "/api/cs/notices?limit=10", "", cs, nil), http.StatusOK, "通知记录（客服）")
	extStatus(t, extDo(t, r, "GET", "/api/cs/orders/"+orderNo, "", cs, nil), http.StatusOK, "订单详情（客服）")
	extStatus(t, extDo(t, r, "POST", "/api/cs/channel/mode", `{"refundMode":"success"}`, cs, nil), http.StatusOK, "渠道退款模式")

	// 渠道侧：待支付订单、渠道状态、渠道代付、强制过期
	rr = extDo(t, r, "POST", "/api/orders", `{"product_id":"SKU-B","quantity":1}`, buyer, nil)
	extStatus(t, rr, http.StatusOK, "下单（待付款）")
	pending := extJSON(t, rr)
	pendingNo, _ := pending["order_no"].(string)
	extStatus(t, extDo(t, r, "GET", "/api/pay/pending", "", "", nil), http.StatusOK, "渠道待支付订单")
	extStatus(t, extDo(t, r, "GET", "/api/pay/channel", "", "", nil), http.StatusOK, "渠道状态")
	extStatus(t, extDo(t, r, "POST", "/api/pay/channel-mode", `{"refund_mode":"success"}`, "", nil), http.StatusOK, "渠道模式（契约）")
	rr = extDo(t, r, "POST", "/api/pay/channel-pay", `{"order_no":"`+pendingNo+`","channel_txn_no":"TXN-CH-1","amount":`+jsonNum(amount)+`}`, "", nil)
	if rr.Code != http.StatusOK && rr.Code != http.StatusConflict && rr.Code != http.StatusBadRequest {
		t.Fatalf("渠道代付异常状态: %d %s", rr.Code, rr.Body.String())
	}
	extStatus(t, extDo(t, r, "POST", "/api/test/expire-orders", `{"order_nos":["`+pendingNo+`"]}`, "", nil), http.StatusOK, "强制过期钩子")
	extStatus(t, extDo(t, r, "POST", "/api/jobs/close-expired", "", "", nil), http.StatusOK, "超时关单")
	extStatus(t, extDo(t, r, "POST", "/api/demo/reset", "", "", nil), http.StatusOK, "演示数据重置")
}

// TestContractLateAndRefundFlows 覆盖迟到支付转异常、客服处理异常退款、渠道失败与重试。
func TestContractLateAndRefundFlows(t *testing.T) {
	r, _ := extSetup(t)

	buyer := extLogin(t, r, "buyer1")
	cs := extLogin(t, r, "cs1")

	// 1. 退款：渠道首次失败（fail）-> 审核返回 409 + 退款单 -> 切成功 -> 重试成功
	extStatus(t, extDo(t, r, "POST", "/api/pay/channel-mode", `{"refund_mode":"fail"}`, "", nil), http.StatusOK, "渠道置失败")
	orderNo, amount := extPaidOrder(t, r, buyer, "TXN-RF-1")
	rr := extDo(t, r, "POST", "/api/orders/"+orderNo+"/refund-apply", `{"reason":"退货退款"}`, buyer, nil)
	extStatus(t, rr, http.StatusOK, "契约退款申请")
	refundNo := ""
	if refund, ok := extJSON(t, rr)["refund"].(map[string]interface{}); ok {
		refundNo, _ = refund["refund_no"].(string)
	}
	if refundNo == "" {
		t.Fatalf("退款申请未返回退款单号: %s", rr.Body.String())
	}
	rr = extDo(t, r, "POST", "/api/cs/refunds/"+refundNo+"/decide", `{"approve":true,"reason":"同意"}`, cs, nil)
	extStatus(t, rr, http.StatusOK, "审核-渠道失败仍受理并留痕")
	if st, _ := extJSON(t, rr)["status"].(string); st != "FAILED" {
		t.Fatalf("渠道失败退款单状态应为 FAILED，实际 %v body=%s", st, rr.Body.String())
	}
	extStatus(t, extDo(t, r, "POST", "/api/pay/channel-mode", `{"refund_mode":"success"}`, "", nil), http.StatusOK, "渠道置成功")
	extStatus(t, extDo(t, r, "POST", "/api/cs/refunds/"+refundNo+"/retry", `{}`, cs, nil), http.StatusOK, "退款重试成功")

	// 2. 迟到支付：待付款订单被强制过期 -> 支付通知转异常收款 -> 客服发起退款
	rr = extDo(t, r, "POST", "/api/orders", `{"product_id":"SKU-B","quantity":1}`, buyer, nil)
	extStatus(t, rr, http.StatusOK, "下单（迟到支付用例）")
	late := extJSON(t, rr)
	lateNo, _ := late["order_no"].(string)
	lateAmount := int64(0)
	if f, ok := late["amount"].(float64); ok {
		lateAmount = int64(f)
	}
	extStatus(t, extDo(t, r, "POST", "/api/test/expire-orders", `{"order_nos":["`+lateNo+`"]}`, "", nil), http.StatusOK, "强制过期")
	extStatus(t, extDo(t, r, "POST", "/api/jobs/close-expired", "", "", nil), http.StatusOK, "关单")

	body := `{"order_no":"` + lateNo + `","channel_txn_no":"TXN-LATE-9","amount":` + jsonNum(lateAmount) + `}`
	rr = extDo(t, r, "POST", "/api/pay/notify", body, "", map[string]string{
		"X-Channel-Signature": extSign(extCfg.Shop.PayNotifySecret, []byte(body)),
	})
	extStatus(t, rr, http.StatusOK, "迟到支付通知")
	res := extJSON(t, rr)
	if status, _ := res["status"].(string); status != "ABNORMAL" && status != "DUPLICATED" {
		t.Fatalf("迟到支付应转异常收款，实际 %v body=%s", res["status"], rr.Body.String())
	}

	// 3. 验签失败 / 金额不符 / 未知订单：均被拒且留痕
	extStatus(t, extDo(t, r, "POST", "/api/pay/notify", body, "", map[string]string{"X-Channel-Signature": "deadbeef"}), http.StatusBadRequest, "验签失败")
	unknownBody := `{"order_no":"SH-NOT-EXIST","channel_txn_no":"X-9","amount":100}`
	extStatus(t, extDo(t, r, "POST", "/api/pay/notify", unknownBody, "", map[string]string{
		"X-Channel-Signature": extSign(extCfg.Shop.PayNotifySecret, []byte(unknownBody)),
	}), http.StatusBadRequest, "未知订单通知被拒")
	_ = amount
}
