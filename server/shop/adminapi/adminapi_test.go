package adminapi_test

// 说明：GVA 管理端商城后台接口（运营/仓库/客服）测试。
// 鉴权由 gva 中间件负责（PrivateGroup + JWT + Casbin），这里用中间件注入 claims
// 模拟已登录的管理员，覆盖 adminapi 三个分组的成功/失败分支与 service 装配。
// 使用真实 MySQL（shop_test，须 -p 1 串行）。

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	systemReq "shop/model/system/request"
	"shop/shop/config"
	shopdb "shop/shop/db"
	"shop/shop/migrations"
	shoprouter "shop/shop/router"
	"shop/shop/service"
	"shop/shop/utils"
)

var (
	testDB  *gorm.DB
	testCfg config.Config
)

// adminID 模拟已登录的 GVA 管理员用户
const adminID = 888

func TestMain(m *testing.M) {
	dsn := os.Getenv("SHOP_TEST_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3307)/shop_test?charset=utf8mb4&parseTime=True&loc=Local"
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
		// sessions 先清：fk_sessions_user 引用 users（共享测试库须防跨包残留）
		"sessions",
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

// newAdminServer 组装带「已登录管理员」的引擎：claims 注入等价于 gva JWT 中间件产物。
// 返回可调时钟指针，便于构造「订单已超时关闭后收到支付通知」的迟到支付场景。
func newAdminServer(t *testing.T) (*gin.Engine, *service.ServiceGroup, *time.Time) {
	t.Helper()
	resetTables(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ch := service.NewSimulatedChannel(true)
	clock := time.Now()
	g := service.NewServiceGroup(service.Deps{DB: testDB, Now: func() time.Time { return clock }, Cfg: testCfg, Channel: ch})
	group := r.Group("", func(c *gin.Context) {
		c.Set("claims", &systemReq.CustomClaims{BaseClaims: systemReq.BaseClaims{ID: adminID}})
	})
	shoprouter.InitShopAdminRouter(group, g)
	return r, g, &clock
}

func doJSON(t *testing.T, r *gin.Engine, method, path, body string) map[string]interface{} {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("%s %s: HTTP %d body=%s", method, path, rr.Code, rr.Body.String())
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: 响应不是 JSON: %v", method, path, err)
	}
	return out
}

// wantOK 断言 GVA 成功响应（code=0）
func wantOK(t *testing.T, res map[string]interface{}, ctx string) map[string]interface{} {
	t.Helper()
	if code, _ := res["code"].(float64); code != 0 {
		t.Fatalf("%s: 期望 code=0 实际 %v msg=%v", ctx, res["code"], res["msg"])
	}
	data, _ := res["data"].(map[string]interface{})
	return data
}

// wantFail 断言业务失败响应（code=7，GVA FailWithMessage/FailWithDetailed）
func wantFail(t *testing.T, res map[string]interface{}, ctx string) {
	t.Helper()
	if code, _ := res["code"].(float64); code == 0 {
		t.Fatalf("%s: 期望业务失败，实际成功 data=%v", ctx, res["data"])
	}
}

func firstProductID(t *testing.T, g *service.ServiceGroup) uint64 {
	t.Helper()
	list, err := g.CatalogService.OnSale()
	if err != nil || len(list) == 0 {
		t.Fatalf("取商品失败: %v", err)
	}
	return list[0].ID
}

// buyerID 取种子买家账号 ID（清表重放种子后自增 ID 会变化，不能写死）
func buyerID(t *testing.T, g *service.ServiceGroup) uint64 {
	t.Helper()
	list, err := g.AuthService.Users("buyer")
	if err != nil || len(list) == 0 {
		t.Fatalf("取买家账号失败: %v", err)
	}
	return list[0].ID
}

// paidOrderNo 创建一笔已支付订单，返回订单号、金额、商品 ID
func paidOrderNo(t *testing.T, g *service.ServiceGroup, txn string) (string, int64, uint64) {
	t.Helper()
	pid := firstProductID(t, g)
	order, err := g.OrderService.Create(buyerID(t, g), pid, 1)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, order.OrderNo, txn, order.AmountCents)
	if _, err := g.PaymentService.Notify(order.OrderNo, txn, order.AmountCents, sign, ""); err != nil {
		t.Fatalf("支付通知失败: %v", err)
	}
	return order.OrderNo, order.AmountCents, pid
}

// ---------- 运营 ----------

func TestOpsHandlers(t *testing.T) {
	r, g, _ := newAdminServer(t)

	orderNo, amount, pid := paidOrderNo(t, g, "TXN-OPS-1")

	if data := wantOK(t, doJSON(t, r, "GET", "/shop/ops/summary", ""), "汇总"); data == nil {
		t.Fatal("汇总应返回数据")
	}
	if data := wantOK(t, doJSON(t, r, "GET", "/shop/ops/products", ""), "商品列表"); data == nil {
		t.Logf("商品列表返回非对象（数组）：%v", data)
	}
	wantOK(t, doJSON(t, r, "GET", "/shop/ops/orders?status=paid", ""), "订单列表(按状态)")
	wantOK(t, doJSON(t, r, "GET", "/shop/ops/orders", ""), "订单列表(全量)")

	// 商品维护：调价 + 覆盖可售 + 上下架
	wantOK(t, doJSON(t, r, "PATCH", "/shop/ops/products/"+itoa(pid), `{"priceCents":12300,"available":9}`), "商品调价/库存")
	wantOK(t, doJSON(t, r, "PATCH", "/shop/ops/products/"+itoa(pid), `{"onSale":false}`), "商品下架")
	wantOK(t, doJSON(t, r, "PATCH", "/shop/ops/products/"+itoa(pid), `{"onSale":true,"priceCents":10000}`), "商品上架")

	// 失败分支：非法主键 / 非法 JSON / 不存在商品 / 空入参
	wantFail(t, doJSON(t, r, "PATCH", "/shop/ops/products/abc", `{}`), "非法主键")
	wantFail(t, doJSON(t, r, "PATCH", "/shop/ops/products/"+itoa(pid), `{`), "非法 JSON")
	wantFail(t, doJSON(t, r, "PATCH", "/shop/ops/products/999999", `{"onSale":true}`), "商品不存在")
	wantFail(t, doJSON(t, r, "PATCH", "/shop/ops/products/"+itoa(pid), `{}`), "空入参")

	// 订单核查 / 留痕 / 流水 / 通知 / 异常
	wantOK(t, doJSON(t, r, "GET", "/shop/ops/orders/"+orderNo, ""), "订单核查")
	wantFail(t, doJSON(t, r, "GET", "/shop/ops/orders/SH-NOT-EXIST", ""), "订单核查-不存在")
	wantOK(t, doJSON(t, r, "GET", "/shop/ops/trail?orderNo="+orderNo, ""), "操作留痕")
	wantOK(t, doJSON(t, r, "GET", "/shop/ops/ledger?productId="+itoa(pid), ""), "库存流水")
	wantOK(t, doJSON(t, r, "GET", "/shop/ops/notices?orderNo="+orderNo, ""), "通知按单号")
	wantOK(t, doJSON(t, r, "GET", "/shop/ops/notices?result=SUCCESS&limit=5", ""), "通知按结果")
	wantOK(t, doJSON(t, r, "GET", "/shop/ops/abnormals?status=pending", ""), "异常收款列表")

	_ = amount
}

// ---------- 仓库 ----------

func TestWarehouseHandlers(t *testing.T) {
	r, g, _ := newAdminServer(t)

	orderNo, _, _ := paidOrderNo(t, g, "TXN-WH-1")

	wantOK(t, doJSON(t, r, "GET", "/shop/warehouse/shipable", ""), "待发货列表")

	wantFail(t, doJSON(t, r, "POST", "/shop/warehouse/ship", `{`), "发货-非法 JSON")
	wantFail(t, doJSON(t, r, "POST", "/shop/warehouse/ship", `{"orderNo":"SH-NOT-EXIST","carrier":"SF","trackingNo":"1"}`), "发货-订单不存在")

	wantOK(t, doJSON(t, r, "POST", "/shop/warehouse/ship",
		`{"orderNo":"`+orderNo+`","carrier":"顺丰","trackingNo":"SF123456789"}`), "发货登记")
	wantFail(t, doJSON(t, r, "POST", "/shop/warehouse/ship",
		`{"orderNo":"`+orderNo+`","carrier":"顺丰","trackingNo":"SF123456789"}`), "重复发货")

	wantOK(t, doJSON(t, r, "GET", "/shop/warehouse/shipments", ""), "发货记录")
	wantOK(t, doJSON(t, r, "GET", "/shop/warehouse/orders/"+orderNo, ""), "订单详情（仓库）")
	wantFail(t, doJSON(t, r, "GET", "/shop/warehouse/orders/SH-NOT-EXIST", ""), "订单详情-不存在")
}

// ---------- 客服 ----------

func TestCsHandlers(t *testing.T) {
	r, g, clk := newAdminServer(t)

	// 渠道开关：失败模式 -> 审核退款失败可重试 -> 成功模式重试成功
	wantFail(t, doJSON(t, r, "POST", "/shop/cs/channel/mode", `{`), "渠道模式-非法 JSON")
	wantOK(t, doJSON(t, r, "POST", "/shop/cs/channel/mode", `{"refundMode":"fail"}`), "渠道模式-失败")

	orderNo, _, _ := paidOrderNo(t, g, "TXN-CS-1")
	refund, err := g.RefundService.Apply(buyerID(t, g), orderNo, "买家申请退款")
	if err != nil {
		t.Fatalf("退款申请失败: %v", err)
	}

	wantOK(t, doJSON(t, r, "GET", "/shop/cs/refunds?status=pending", ""), "退款单列表")
	wantFail(t, doJSON(t, r, "POST", "/shop/cs/refunds/audit", `{`), "审核-非法 JSON")
	// 渠道失败：审核已生效但渠道调用失败，返回 code!=0 且带退款单
	wantFail(t, doJSON(t, r, "POST", "/shop/cs/refunds/audit",
		`{"orderNo":"`+orderNo+`","approve":true,"reason":"同意整单退款"}`), "审核-渠道失败")

	wantOK(t, doJSON(t, r, "POST", "/shop/cs/channel/mode", `{"refundMode":"success"}`), "渠道模式-成功")
	wantOK(t, doJSON(t, r, "POST", "/shop/cs/refunds/retry", `{"refundNo":"`+refund.RefundNo+`"}`), "退款重试")
	wantFail(t, doJSON(t, r, "POST", "/shop/cs/refunds/retry", `{"refundNo":"RF-NOT-EXIST"}`), "重试-退款单不存在")

	// 售前拒绝分支：新订单 -> 申请 -> 拒绝（订单回到待发货）
	orderNo2, _, _ := paidOrderNo(t, g, "TXN-CS-2")
	if _, err := g.RefundService.Apply(buyerID(t, g), orderNo2, "临时申请"); err != nil {
		t.Fatalf("退款申请失败: %v", err)
	}
	wantOK(t, doJSON(t, r, "POST", "/shop/cs/refunds/audit",
		`{"orderNo":"`+orderNo2+`","approve":false,"reason":"发货在即"}`), "审核-拒绝")

	// 迟到支付 -> 异常收款 -> 客服发起退款（推进时钟 + 服务端关单构造真实场景）
	orderNo3, err := g.OrderService.Create(buyerID(t, g), firstProductID(t, g), 1)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	*clk = clk.Add(11 * time.Minute)
	if _, err := g.OrderService.CloseExpired(); err != nil {
		t.Fatalf("超时关单失败: %v", err)
	}
	txn := "TXN-LATE-1"
	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, orderNo3.OrderNo, txn, orderNo3.AmountCents)
	res, err := g.PaymentService.Notify(orderNo3.OrderNo, txn, orderNo3.AmountCents, sign, "")
	if err != nil {
		t.Fatalf("迟到支付通知失败: %v", err)
	}
	if res.Result != "abnormal" { // model.NoticeAbnormal
		t.Fatalf("迟到支付应转异常收款，实际 %s（%s）", res.Result, res.Reason)
	}

	wantOK(t, doJSON(t, r, "GET", "/shop/cs/abnormals?status=pending", ""), "异常收款列表（客服）")
	wantFail(t, doJSON(t, r, "POST", "/shop/cs/abnormals/handle", `{`), "异常处理-非法 JSON")
	wantFail(t, doJSON(t, r, "POST", "/shop/cs/abnormals/handle",
		`{"channelTxnId":"TXN-NOT-EXIST","reason":"x"}`), "异常处理-交易号不存在")
	wantOK(t, doJSON(t, r, "POST", "/shop/cs/abnormals/handle",
		`{"channelTxnId":"`+txn+`","reason":"核实后原路退回"}`), "异常处理-发起退款")

	wantOK(t, doJSON(t, r, "GET", "/shop/cs/notices?limit=10", ""), "通知记录（客服）")
	wantOK(t, doJSON(t, r, "GET", "/shop/cs/notices?result=ABNORMAL&limit=10", ""), "通知记录-按结果")
	wantOK(t, doJSON(t, r, "GET", "/shop/cs/orders/"+orderNo, ""), "订单详情（客服）")
	wantFail(t, doJSON(t, r, "GET", "/shop/cs/orders/SH-NOT-EXIST", ""), "订单详情-不存在")
}

func itoa(v uint64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
