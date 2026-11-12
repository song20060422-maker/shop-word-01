package router

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/config"
	shopdb "github.com/flipped-aurora/gin-vue-admin/server/shop/db"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/migrations"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/service"
)

// 说明：HTTP 集成测试使用真实 MySQL（本机 3307，独立库 shop_test_router），
// 覆盖 router 装配、JWT/角色门禁、契约兼容层与内置页面。
// 数据库可用 SHOP_TEST_DSN 覆盖。

var testDB *gorm.DB
var testCfg config.Config

func TestMain(m *testing.M) {
	dsn := os.Getenv("SHOP_TEST_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3307)/shop_test_router?charset=utf8mb4&parseTime=True&loc=Local"
	}
	testCfg = config.Default()
	testCfg.Mysql.DSN = dsn
	testCfg.Shop.PayNotifySecret = "test-secret"
	testCfg.Shop.OrderTimeoutMinutes = 10
	db, err := shopdb.Gorm(testCfg)
	if err != nil {
		panic("连接测试库失败（请确认 MySQL 3307 可用）: " + err.Error())
	}
	if err := shopdb.Migrate(db, migrations.FS); err != nil {
		panic("测试库迁移失败: " + err.Error())
	}
	if err := shopdb.SeedAccounts(db, "shop123"); err != nil {
		panic("种子账号失败: " + err.Error())
	}
	testDB = db
	os.Exit(m.Run())
}

func resetTables(t *testing.T) {
	t.Helper()
	tables := []string{
		"abnormal_payments", "payments", "payment_notices", "refunds", "shipments",
		"inventory_ledger", "operation_logs", "orders", "products", "users",
	}
	for _, tbl := range tables {
		if err := testDB.Exec("DELETE FROM " + tbl).Error; err != nil {
			t.Fatalf("清理 %s 失败: %v", tbl, err)
		}
	}
	if err := shopdb.Migrate(testDB, migrations.FS); err != nil {
		t.Fatalf("重放迁移失败: %v", err)
	}
	if err := shopdb.SeedAccounts(testDB, "shop123"); err != nil {
		t.Fatalf("重放种子账号失败: %v", err)
	}
}

func newServer(t *testing.T, demoMode bool) (*gin.Engine, *service.SimulatedChannel) {
	t.Helper()
	resetTables(t)
	cfg := testCfg
	cfg.Shop.DemoMode = demoMode
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ch := service.NewSimulatedChannel(true)
	g := service.NewServiceGroup(service.Deps{DB: testDB, Now: time.Now, Cfg: cfg, Channel: ch})
	InitShopRouter(r.Group(""), g, cfg, time.Now)
	return r, ch
}

func doJSON(r *gin.Engine, method, path, body, token string, headers map[string]string) *httptest.ResponseRecorder {
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

func parseBody(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是 JSON: %v body=%s", err, rr.Body.String())
	}
	return out
}

func login(t *testing.T, r *gin.Engine, username string) string {
	t.Helper()
	rr := doJSON(r, "POST", "/api/login", `{"username":"`+username+`","password":"shop123"}`, "", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("登录 %s 失败: %d %s", username, rr.Code, rr.Body.String())
	}
	res := parseBody(t, rr)
	token, _ := res["token"].(string)
	if token == "" {
		t.Fatalf("登录 %s 未返回 token: %s", username, rr.Body.String())
	}
	return token
}

func statusOK(t *testing.T, rr *httptest.ResponseRecorder, want int, ctx string) {
	t.Helper()
	if rr.Code != want {
		t.Fatalf("%s: 期望 %d 实际 %d body=%s", ctx, want, rr.Code, rr.Body.String())
	}
}

func paySignRaw(secret string, raw []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	return hex.EncodeToString(mac.Sum(nil))
}

// compatPayNotify 走契约通道发送支付通知（HMAC 对原始请求体）。
func compatPayNotify(t *testing.T, r *gin.Engine, orderNo, txn string, amount int64, signHeader string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"order_no":"` + orderNo + `","channel_txn_no":"` + txn + `","amount":` + itoa(amount) + `}`
	sig := paySignRaw(testCfg.Shop.PayNotifySecret, []byte(body))
	if signHeader == "bad" {
		sig = "deadbeef"
	}
	return doJSON(r, "POST", "/api/pay/notify", body, "", map[string]string{"X-Channel-Signature": sig})
}

func itoa(v int64) string {
	return json.Number(jsonInt(v)).String()
}

func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// 创建一笔已支付订单，返回订单号与金额。
func paidOrder(t *testing.T, r *gin.Engine, buyerToken, txn string) (string, int64) {
	t.Helper()
	rr := doJSON(r, "POST", "/api/orders", `{"product_id":"SKU-A","quantity":2}`, buyerToken, nil)
	statusOK(t, rr, http.StatusOK, "下单")
	res := parseBody(t, rr)
	orderNo, _ := res["order_no"].(string)
	amount := int64(numOf(res["amount"]))
	if orderNo == "" || amount <= 0 {
		t.Fatalf("下单响应缺少字段: %s", rr.Body.String())
	}
	rr = compatPayNotify(t, r, orderNo, txn, amount, "")
	statusOK(t, rr, http.StatusOK, "支付通知")
	res = parseBody(t, rr)
	if res["status"] != "SUCCESS" {
		t.Fatalf("支付通知未受理: %s", rr.Body.String())
	}
	return orderNo, amount
}

func numOf(v interface{}) float64 {
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

func TestPublicPagesAndAuth(t *testing.T) {
	r, _ := newServer(t, false)

	rr := doJSON(r, "GET", "/api/health", "", "", nil)
	statusOK(t, rr, http.StatusOK, "健康检查")
	if res := parseBody(t, rr); res["status"] != "ok" {
		t.Fatalf("健康检查内容错误: %s", rr.Body.String())
	}

	rr = doJSON(r, "GET", "/api/users", "", "", nil)
	statusOK(t, rr, http.StatusOK, "账号列表")

	rr = doJSON(r, "GET", "/api/products", "", "", nil)
	statusOK(t, rr, http.StatusOK, "商品列表")
	res := parseBody(t, rr)
	if list, ok := res["products"].([]interface{}); !ok || len(list) != 2 {
		t.Fatalf("在售商品应为 2 个: %s", rr.Body.String())
	}

	rr = doJSON(r, "GET", "/shop", "", "", nil)
	statusOK(t, rr, http.StatusOK, "内置页面")
	if ct := rr.Header().Get("Content-Type"); ct == "" {
		t.Fatal("内置页面缺少 Content-Type")
	}

	// 登录：成功 / 密码错误 / 员工契约别名
	rr = doJSON(r, "POST", "/api/login", `{"username":"buyer1","password":"shop123"}`, "", nil)
	statusOK(t, rr, http.StatusOK, "买家登录")
	rr = doJSON(r, "POST", "/api/login", `{"username":"buyer1","password":"wrong"}`, "", nil)
	if rr.Code < 400 {
		t.Fatalf("错误密码应拒绝: %d", rr.Code)
	}
	rr = doJSON(r, "POST", "/api/login", `{"username":"ops","password":"shop123"}`, "", nil)
	statusOK(t, rr, http.StatusOK, "契约员工别名登录")
	if res := parseBody(t, rr); res["role"] != "ops" {
		t.Fatalf("别名登录角色错误: %s", rr.Body.String())
	}

	// 未登录 / 伪造令牌
	rr = doJSON(r, "GET", "/api/me", "", "", nil)
	statusOK(t, rr, http.StatusUnauthorized, "未登录访问")
	rr = doJSON(r, "GET", "/api/me", "", "not-a-token", nil)
	statusOK(t, rr, http.StatusUnauthorized, "伪造令牌")

	token := login(t, r, "buyer1")
	rr = doJSON(r, "GET", "/api/me", "", token, nil)
	statusOK(t, rr, http.StatusOK, "当前用户")
	if me, ok := parseBody(t, rr)["data"].(map[string]interface{}); !ok || me["username"] != "buyer1" {
		t.Fatalf("/api/me 用户错误: %s", rr.Body.String())
	}
}

func TestBuyerOrderAndPayFlow(t *testing.T) {
	r, _ := newServer(t, false)
	token := login(t, r, "buyer1")

	rr := doJSON(r, "POST", "/api/orders", `{"product_id":"SKU-A","quantity":2}`, token, nil)
	statusOK(t, rr, http.StatusOK, "下单")
	res := parseBody(t, rr)
	orderNo, _ := res["order_no"].(string)
	if orderNo == "" || res["status"] != "WAIT_PAY" {
		t.Fatalf("下单响应错误: %s", rr.Body.String())
	}
	amount := int64(numOf(res["amount"]))
	if list, ok := res["data"].(map[string]interface{}); !ok || list["order"] == nil {
		t.Fatalf("下单响应缺少 data 副本: %s", rr.Body.String())
	}

	rr = doJSON(r, "GET", "/api/orders", "", token, nil)
	statusOK(t, rr, http.StatusOK, "我的订单")
	rr = doJSON(r, "GET", "/api/orders/"+orderNo, "", token, nil)
	statusOK(t, rr, http.StatusOK, "订单详情")
	rr = doJSON(r, "GET", "/api/orders/NO-SUCH-ORDER", "", token, nil)
	statusOK(t, rr, http.StatusNotFound, "未知订单详情")
	rr = doJSON(r, "GET", "/api/orders/"+orderNo, "", login(t, r, "buyer2"), nil)
	statusOK(t, rr, http.StatusForbidden, "他人订单详情")

	rr = doJSON(r, "GET", "/api/pay/pending", "", "", nil)
	statusOK(t, rr, http.StatusOK, "渠道待支付列表")

	// 有效支付通知（契约通道）
	rr = compatPayNotify(t, r, orderNo, "TXN-1", amount, "")
	statusOK(t, rr, http.StatusOK, "支付通知")
	// 重复通知 → 409 且不重复记款
	rr = compatPayNotify(t, r, orderNo, "TXN-1", amount, "")
	if rr.Code != http.StatusConflict {
		t.Fatalf("重复通知应 409: %d %s", rr.Code, rr.Body.String())
	}
	// 验签失败 → 4xx
	rr = compatPayNotify(t, r, orderNo, "TXN-2", amount, "bad")
	if rr.Code < 400 {
		t.Fatalf("验签失败应拒绝: %d", rr.Code)
	}
	// 渠道受理付款：返回交易号，可用于后续通知
	rr = doJSON(r, "POST", "/api/pay/channel-pay", `{"order_no":"`+orderNo+`"}`, "", nil)
	statusOK(t, rr, http.StatusOK, "渠道受理付款")
	if res := parseBody(t, rr); res["status"] != "SUCCESS" {
		t.Fatalf("渠道受理响应错误: %s", rr.Body.String())
	}

	// 取消待付款订单
	rr = doJSON(r, "POST", "/api/orders", `{"product_id":"SKU-B","quantity":1}`, token, nil)
	res = parseBody(t, rr)
	no2, _ := res["order_no"].(string)
	rr = doJSON(r, "POST", "/api/orders/"+no2+"/cancel", "", token, nil)
	statusOK(t, rr, http.StatusOK, "取消订单")
	rr = doJSON(r, "POST", "/api/orders/"+no2+"/cancel", "", token, nil)
	if rr.Code < 400 {
		t.Fatalf("重复取消应拒绝: %d", rr.Code)
	}

	// 契约独立退款申请路径（已付款订单）
	rr = doJSON(r, "POST", "/api/orders", `{"product_id":"SKU-B","quantity":1}`, token, nil)
	res = parseBody(t, rr)
	no3, _ := res["order_no"].(string)
	amount3 := int64(numOf(res["amount"]))
	rr = compatPayNotify(t, r, no3, "TXN-REFUND-APPLY", amount3, "")
	statusOK(t, rr, http.StatusOK, "第三单支付")
	rr = doJSON(r, "POST", "/api/orders/"+no3+"/refund-apply", `{"reason":"契约退款"}`, token, nil)
	statusOK(t, rr, http.StatusOK, "契约退款申请")
	if res := parseBody(t, rr); res["refund_id"] == nil || res["refund_id"] == "" {
		t.Fatalf("退款申请缺少 refund_id: %s", rr.Body.String())
	}
	// 未登录申请退款 → 401
	rr = doJSON(r, "POST", "/api/orders/"+no3+"/refund-apply", `{"reason":"x"}`, "", nil)
	statusOK(t, rr, http.StatusUnauthorized, "未登录退款申请")
}

func TestWarehouseShipAndRoleGuard(t *testing.T) {
	r, _ := newServer(t, false)
	buyer := login(t, r, "buyer1")
	wh := login(t, r, "wh1")
	orderNo, amount := paidOrder(t, r, buyer, "TXN-SHIP-1")

	rr := doJSON(r, "GET", "/api/warehouse/orders", "", wh, nil)
	statusOK(t, rr, http.StatusOK, "可发货列表(契约)")
	rr = doJSON(r, "GET", "/api/warehouse/shipable", "", wh, nil)
	statusOK(t, rr, http.StatusOK, "可发货列表(原型)")

	rr = doJSON(r, "POST", "/api/warehouse/orders/"+orderNo+"/ship", `{"carrier":"顺丰","tracking_no":"SF100"}`, wh, nil)
	statusOK(t, rr, http.StatusOK, "发货登记")
	res := parseBody(t, rr)
	if res["status"] != "SHIPPED" || res["tracking_no"] != "SF100" {
		t.Fatalf("发货响应错误: %s", rr.Body.String())
	}
	rr = doJSON(r, "POST", "/api/warehouse/orders/"+orderNo+"/ship", `{"carrier":"顺丰","tracking_no":"SF101"}`, wh, nil)
	if rr.Code < 400 {
		t.Fatalf("重复发货应拒绝: %d", rr.Code)
	}
	rr = doJSON(r, "GET", "/api/warehouse/shipments", "", wh, nil)
	statusOK(t, rr, http.StatusOK, "发货记录")

	// 原型发货入口（POST /api/warehouse/ship，request.Ship 结构）
	orderNo2, amount2 := paidOrder(t, r, buyer, "TXN-SHIP-2")
	rr = doJSON(r, "POST", "/api/warehouse/ship",
		`{"orderNo":"`+orderNo2+`","carrier":"SF","trackingNo":"SF300"}`, wh, nil)
	statusOK(t, rr, http.StatusOK, "原型发货登记")
	if data, ok := parseBody(t, rr)["data"].(map[string]interface{}); !ok || data["status"] != "shipped" {
		t.Fatalf("原型发货状态错误: %s", rr.Body.String())
	}
	_ = amount2

	// 买家访问仓库接口 → 403
	rr = doJSON(r, "GET", "/api/warehouse/orders", "", buyer, nil)
	statusOK(t, rr, http.StatusForbidden, "买家访问仓库接口")
	_ = amount
}

func TestOpsMaintainAndReport(t *testing.T) {
	r, _ := newServer(t, false)
	buyer := login(t, r, "buyer1")
	ops := login(t, r, "ops1")
	orderNo, _ := paidOrder(t, r, buyer, "TXN-OPS-1")

	rr := doJSON(r, "PATCH", "/api/ops/products/SKU-A", `{"price_cents":5200}`, ops, nil)
	statusOK(t, rr, http.StatusOK, "调价")
	rr = doJSON(r, "PATCH", "/api/ops/products/SKU-A", `{"available":9}`, ops, nil)
	statusOK(t, rr, http.StatusOK, "覆盖可售")
	if res := parseBody(t, rr); int(numOf(res["available"])) != 9 {
		t.Fatalf("覆盖可售失败: %s", rr.Body.String())
	}
	rr = doJSON(r, "PATCH", "/api/ops/products/SKU-A", `{}`, ops, nil)
	if rr.Code < 400 {
		t.Fatalf("空更新应拒绝: %d", rr.Code)
	}
	rr = doJSON(r, "PATCH", "/api/ops/products/SKU-A", `{"price_cents":5200}`, buyer, nil)
	statusOK(t, rr, http.StatusForbidden, "买家维护商品")

	// 原型维护入口：productId + priceCents（productId 从商品列表读取，避免依赖自增起始值）
	rr = doJSON(r, "GET", "/api/products", "", "", nil)
	plist := parseBody(t, rr)["products"].([]interface{})
	pid := json.Number(jsonInt(int64(numOf(plist[0].(map[string]interface{})["productId"])))).String()
	rr = doJSON(r, "PATCH", "/api/ops/products", `{"productId":`+pid+`,"priceCents":5300}`, ops, nil)
	statusOK(t, rr, http.StatusOK, "原型维护商品")

	rr = doJSON(r, "GET", "/api/ops/summary", "", ops, nil)
	statusOK(t, rr, http.StatusOK, "交易汇总")
	rr = doJSON(r, "GET", "/api/ops/orders", "", ops, nil)
	statusOK(t, rr, http.StatusOK, "订单列表")
	rr = doJSON(r, "GET", "/api/ops/orders/"+orderNo, "", ops, nil)
	statusOK(t, rr, http.StatusOK, "订单全链路")
	rr = doJSON(r, "GET", "/api/ops/notices?orderNo="+orderNo, "", ops, nil)
	statusOK(t, rr, http.StatusOK, "通知查询")
	rr = doJSON(r, "GET", "/api/ops/notices?result=rejected", "", ops, nil)
	statusOK(t, rr, http.StatusOK, "通知按结果过滤")
	rr = doJSON(r, "GET", "/api/ops/abnormals", "", ops, nil)
	statusOK(t, rr, http.StatusOK, "异常列表")
	rr = doJSON(r, "GET", "/api/ops/ledger?productId=1", "", ops, nil)
	statusOK(t, rr, http.StatusOK, "库存流水")
	rr = doJSON(r, "GET", "/api/ops/trail?orderNo="+orderNo, "", ops, nil)
	statusOK(t, rr, http.StatusOK, "操作轨迹")
	rr = doJSON(r, "GET", "/api/ops/payments?order_no="+orderNo, "", ops, nil)
	statusOK(t, rr, http.StatusOK, "支付记录查询")
	if res := parseBody(t, rr); len(res["payments"].([]interface{})) == 0 {
		t.Fatalf("支付记录为空: %s", rr.Body.String())
	}
	// 买家访问运营接口 → 403
	rr = doJSON(r, "GET", "/api/ops/summary", "", buyer, nil)
	statusOK(t, rr, http.StatusForbidden, "买家访问运营接口")
}

func TestRefundFlow(t *testing.T) {
	r, ch := newServer(t, false)
	buyer := login(t, r, "buyer1")
	wh := login(t, r, "wh1")
	cs := login(t, r, "cs1")

	// 场景一：申请 → 发货被拒 → 客服拒绝 → 恢复发货
	orderNo, _ := paidOrder(t, r, buyer, "TXN-RF-1")
	rr := doJSON(r, "POST", "/api/orders/"+orderNo+"/refund", `{"reason":"不想要了"}`, buyer, nil)
	statusOK(t, rr, http.StatusOK, "退款申请")
	rr = doJSON(r, "GET", "/api/orders/"+orderNo+"/refund", "", buyer, nil)
	statusOK(t, rr, http.StatusOK, "买家查退款")
	rr = doJSON(r, "POST", "/api/warehouse/orders/"+orderNo+"/ship", `{"carrier":"SF","tracking_no":"SF200"}`, wh, nil)
	if rr.Code < 400 {
		t.Fatalf("退款申请期发货应被拒: %d", rr.Code)
	}
	rr = doJSON(r, "GET", "/api/cs/refunds?status=pending", "", cs, nil)
	statusOK(t, rr, http.StatusOK, "客服退款列表")
	res := parseBody(t, rr)
	list := res["refunds"].([]interface{})
	first := list[0].(map[string]interface{})
	refundID, _ := first["id"].(string)
	rr = doJSON(r, "POST", "/api/cs/refunds/"+refundID+"/decide", `{"approve":false,"reason":"凭证不足"}`, cs, nil)
	statusOK(t, rr, http.StatusOK, "客服拒绝退款")
	if res := parseBody(t, rr); res["status"] != "REJECTED" {
		t.Fatalf("拒绝后状态错误: %s", rr.Body.String())
	}
	rr = doJSON(r, "POST", "/api/warehouse/orders/"+orderNo+"/ship", `{"carrier":"SF","tracking_no":"SF201"}`, wh, nil)
	statusOK(t, rr, http.StatusOK, "拒绝后可发货")

	// 场景二：申请 → 同意 → 渠道失败 → 重试成功
	orderNo2, _ := paidOrder(t, r, buyer, "TXN-RF-2")
	rr = doJSON(r, "POST", "/api/orders/"+orderNo2+"/refund", `{"reason":"换货"}`, buyer, nil)
	statusOK(t, rr, http.StatusOK, "第二笔退款申请")
	rr = doJSON(r, "POST", "/api/pay/channel-mode", `{"refund_result":"FAIL"}`, "", nil)
	statusOK(t, rr, http.StatusOK, "渠道切失败")
	rr = doJSON(r, "GET", "/api/cs/refunds?status=pending", "", cs, nil)
	list = parseBody(t, rr)["refunds"].([]interface{})
	rid2 := list[0].(map[string]interface{})["id"].(string)
	rr = doJSON(r, "POST", "/api/cs/refunds/"+rid2+"/decide", `{"approve":true}`, cs, nil)
	statusOK(t, rr, http.StatusOK, "同意后渠道失败仍 2xx")
	if res := parseBody(t, rr); res["status"] != "FAILED" {
		t.Fatalf("渠道失败状态错误: %s", rr.Body.String())
	}
	rr = doJSON(r, "POST", "/api/cs/refunds/"+rid2+"/retry", `{}`, cs, nil)
	if rr.Code == http.StatusOK {
		t.Fatalf("渠道仍失败时重试不应成功: %s", rr.Body.String())
	}
	// 原型重试入口（POST /api/cs/refunds/retry，request.RefundRetry 结构）同样返回冲突
	rr = doJSON(r, "POST", "/api/cs/refunds/retry", `{"refundNo":"`+rid2+`"}`, cs, nil)
	if rr.Code == http.StatusOK {
		t.Fatalf("原型重试仍失败时不应成功: %s", rr.Body.String())
	}
	rr = doJSON(r, "POST", "/api/pay/channel-mode", `{"refund_result":"SUCCESS"}`, "", nil)
	statusOK(t, rr, http.StatusOK, "渠道切成功")
	rr = doJSON(r, "POST", "/api/cs/refunds/"+rid2+"/retry", `{}`, cs, nil)
	statusOK(t, rr, http.StatusOK, "重试成功")
	if res := parseBody(t, rr); res["status"] != "REFUNDED" {
		t.Fatalf("重试后状态错误: %s", rr.Body.String())
	}
	rr = doJSON(r, "GET", "/api/orders/"+orderNo2+"/refund", "", buyer, nil)
	statusOK(t, rr, http.StatusOK, "买家查退款终态")
	if data, ok := parseBody(t, rr)["data"].(map[string]interface{}); !ok || data["channelStatus"] != "success" {
		t.Fatalf("买家可见退款终态错误: %s", rr.Body.String())
	}

	// 场景三：原型审核入口 RefundAudit（同单重复审核 → 409）
	orderNo3, _ := paidOrder(t, r, buyer, "TXN-RF-3")
	rr = doJSON(r, "POST", "/api/orders/"+orderNo3+"/refund", `{"reason":"测试"}`, buyer, nil)
	statusOK(t, rr, http.StatusOK, "第三笔退款申请")
	rr = doJSON(r, "POST", "/api/cs/refunds/audit", `{"orderNo":"`+orderNo3+`","approve":true,"reason":"ok"}`, cs, nil)
	statusOK(t, rr, http.StatusOK, "原型审核同意")
	rr = doJSON(r, "POST", "/api/cs/refunds/audit", `{"orderNo":"`+orderNo3+`","approve":true,"reason":"ok"}`, cs, nil)
	if rr.Code < 400 {
		t.Fatalf("重复审核应拒绝: %d", rr.Code)
	}

	// 渠道状态查询 + 非法模式
	rr = doJSON(r, "GET", "/api/pay/channel", "", "", nil)
	statusOK(t, rr, http.StatusOK, "渠道状态")
	rr = doJSON(r, "POST", "/api/pay/channel-mode", `{"refund_result":"BAD"}`, "", nil)
	if rr.Code < 400 {
		t.Fatalf("非法渠道模式应拒绝: %d", rr.Code)
	}
	rr = doJSON(r, "POST", "/api/cs/channel/mode", `{"refundMode":"fail"}`, cs, nil)
	statusOK(t, rr, http.StatusOK, "原型渠道模式切换")
	ch.SetRefundMode("success")
	_ = ch
}

func TestLatePaymentAbnormal(t *testing.T) {
	r, _ := newServer(t, false)
	buyer := login(t, r, "buyer1")
	cs := login(t, r, "cs1")
	ops := login(t, r, "ops1")

	rr := doJSON(r, "POST", "/api/orders", `{"product_id":"SKU-A","quantity":1}`, buyer, nil)
	res := parseBody(t, rr)
	orderNo, _ := res["order_no"].(string)
	amount := int64(numOf(res["amount"]))

	rr = doJSON(r, "POST", "/api/test/expire-orders", `{}`, "", nil)
	statusOK(t, rr, http.StatusOK, "测试钩子超时关单")
	if res := parseBody(t, rr); int(numOf(res["closed"])) != 1 {
		t.Fatalf("应关闭 1 笔: %s", rr.Body.String())
	}

	// 迟到通知 → ABNORMAL
	rr = compatPayNotify(t, r, orderNo, "TXN-LATE-1", amount, "")
	statusOK(t, rr, http.StatusOK, "迟到通知")
	if res := parseBody(t, rr); res["status"] != "ABNORMAL" {
		t.Fatalf("迟到通知应转异常: %s", rr.Body.String())
	}

	rr = doJSON(r, "GET", "/api/cs/payment-exceptions", "", cs, nil)
	statusOK(t, rr, http.StatusOK, "异常列表")
	list := parseBody(t, rr)["exceptions"].([]interface{})
	ab := list[0].(map[string]interface{})
	abID := json.Number(jsonInt(int64(numOf(ab["id"])))).String()
	rr = doJSON(r, "POST", "/api/cs/payment-exceptions/"+abID+"/refund", `{"reason":"核实迟到支付"}`, cs, nil)
	statusOK(t, rr, http.StatusOK, "异常退款")
	rr = doJSON(r, "POST", "/api/cs/payment-exceptions/"+abID+"/refund", `{"reason":"再次处理"}`, cs, nil)
	if rr.Code < 400 {
		t.Fatalf("已处理异常不应重复退款: %d", rr.Code)
	}

	rr = doJSON(r, "GET", "/api/ops/abnormals?status=handled", "", ops, nil)
	statusOK(t, rr, http.StatusOK, "运营查已处理异常")
	rr = doJSON(r, "POST", "/api/jobs/close-expired", `{}`, "", nil)
	statusOK(t, rr, http.StatusOK, "超时清扫任务")

	// 原型异常处理入口 HandleAbnormal
	rr = doJSON(r, "POST", "/api/orders", `{"product_id":"SKU-B","quantity":1}`, buyer, nil)
	res = parseBody(t, rr)
	no2, _ := res["order_no"].(string)
	amount2 := int64(numOf(res["amount"]))
	doJSON(r, "POST", "/api/test/expire-orders", `{}`, "", nil)
	rr = compatPayNotify(t, r, no2, "TXN-LATE-2", amount2, "")
	statusOK(t, rr, http.StatusOK, "第二笔迟到通知")
	rr = doJSON(r, "POST", "/api/cs/abnormals/handle", `{"channelTxnId":"TXN-LATE-2","reason":"原型入口处理"}`, cs, nil)
	statusOK(t, rr, http.StatusOK, "原型异常处理")

	// 演示模式关闭时禁止重置
	rr = doJSON(r, "POST", "/api/demo/reset", `{}`, "", nil)
	statusOK(t, rr, http.StatusForbidden, "演示模式未开启")
}

func TestDemoResetEnabled(t *testing.T) {
	r, _ := newServer(t, true)
	rr := doJSON(r, "POST", "/api/demo/reset", `{}`, "", nil)
	statusOK(t, rr, http.StatusOK, "演示重置")
	if data, ok := parseBody(t, rr)["data"].(map[string]interface{}); !ok || data["reset"] != true {
		t.Fatalf("重置响应错误: %s", rr.Body.String())
	}
}
