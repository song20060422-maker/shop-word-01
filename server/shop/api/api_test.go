package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shop/shop/config"
	shopdb "shop/shop/db"
	"shop/shop/migrations"
	"shop/shop/service"
	"shop/shop/utils"
)

// 说明：本文件直测被契约兼容层委派遮蔽的原型 handler（同一路径已由 CompatApi 接管），
// 与 router 集成测试互补，保证原型实现同样被覆盖。
// 数据库可用 SHOP_TEST_DSN 覆盖（默认 shop_test，须 -p 1 串行执行）。

var testDB *gorm.DB
var testCfg config.Config

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

func newGroup(t *testing.T) *service.ServiceGroup {
	t.Helper()
	resetTables(t)
	ch := service.NewSimulatedChannel(true)
	return service.NewServiceGroup(service.Deps{DB: testDB, Now: time.Now, Cfg: testCfg, Channel: ch})
}

// invoke 以带 body 与 claims 的测试上下文直调 handler，返回响应记录器。
func invoke(t *testing.T, method, path, body string, claims *utils.Claims, handler gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	return invokeParams(t, method, path, body, claims, handler, nil)
}

func invokeParams(t *testing.T, method, path, body string, claims *utils.Claims, handler gin.HandlerFunc, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if body != "" {
		c.Request = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
	} else {
		c.Request = httptest.NewRequest(method, path, nil)
	}
	if claims != nil {
		c.Set("shopClaims", *claims)
	}
	c.Params = params
	handler(c)
	return w
}

func userIDOf(t *testing.T, g *service.ServiceGroup, username string) uint64 {
	t.Helper()
	login, err := g.AuthService.Login(username, "shop123")
	if err != nil {
		t.Fatalf("登录 %s 失败: %v", username, err)
	}
	return login.User.ID
}

func productRowOf(t *testing.T, g *service.ServiceGroup, sku string) uint64 {
	t.Helper()
	list, err := g.CatalogService.OnSale()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list {
		if p.SKU == sku {
			return p.ID
		}
	}
	t.Fatalf("未找到商品 %s", sku)
	return 0
}

func codeOf(t *testing.T, w *httptest.ResponseRecorder) float64 {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是 JSON: %s", w.Body.String())
	}
	if f, ok := out["code"].(float64); ok {
		return f
	}
	t.Fatalf("响应缺少 code: %s", w.Body.String())
	return -1
}

func TestShadowedPrototypeHandlers(t *testing.T) {
	g := newGroup(t)
	buyer := userIDOf(t, g, "buyer1")
	pid := productRowOf(t, g, "SKU-A")

	// 原型登录（被 CompatApi.Login 遮蔽的原型实现）
	w := invoke(t, "POST", "/api/login", `{"username":"buyer1","password":"shop123"}`, nil, AuthApi{g}.Login)
	if w.Code != http.StatusOK || codeOf(t, w) != 0 {
		t.Fatalf("原型登录失败: %d %s", w.Code, w.Body.String())
	}
	w = invoke(t, "POST", "/api/login", `{"username":"buyer1","password":"wrong"}`, nil, AuthApi{g}.Login)
	if w.Code < 400 {
		t.Fatalf("错误密码应拒绝: %d", w.Code)
	}
	w = invoke(t, "POST", "/api/login", `{bad json`, nil, AuthApi{g}.Login)
	if w.Code < 400 {
		t.Fatalf("非法入参应拒绝: %d", w.Code)
	}

	// 原型商品列表
	w = invoke(t, "GET", "/api/products", "", nil, BuyerApi{g}.Products)
	if w.Code != http.StatusOK {
		t.Fatalf("原型商品列表失败: %d", w.Code)
	}

	// 原型下单
	claims := &utils.Claims{UserID: buyer, Role: "buyer"}
	w = invoke(t, "POST", "/api/orders", `{"productId":`+strconv.FormatUint(pid, 10)+`,"qty":2}`, claims, BuyerApi{g}.CreateOrder)
	if w.Code != http.StatusOK || codeOf(t, w) != 0 {
		t.Fatalf("原型下单失败: %d %s", w.Code, w.Body.String())
	}
	w = invoke(t, "POST", "/api/orders", `{"productId":`+strconv.FormatUint(pid, 10)+`,"qty":5}`, claims, BuyerApi{g}.CreateOrder)
	if w.Code < 400 {
		t.Fatalf("超量下单应拒绝: %d", w.Code)
	}
	w = invoke(t, "POST", "/api/orders", `{bad json`, claims, BuyerApi{g}.CreateOrder)
	if w.Code < 400 {
		t.Fatalf("非法入参应拒绝: %d", w.Code)
	}

	// 原型订单详情（本人可见，他人 403）
	list, _ := g.OrderService.ListByBuyer(buyer, "")
	if len(list) == 0 {
		t.Fatal("下单失败")
	}
	orderNo := list[0].OrderNo
	w = invokeParams(t, "GET", "/api/orders/:orderNo", "", claims, BuyerApi{g}.OrderDetail, gin.Params{{Key: "orderNo", Value: orderNo}})
	if w.Code != http.StatusOK {
		t.Fatalf("原型订单详情失败: %d", w.Code)
	}
	other := &utils.Claims{UserID: userIDOf(t, g, "buyer2"), Role: "buyer"}
	w = invokeParams(t, "GET", "/api/orders/:orderNo", "", other, BuyerApi{g}.OrderDetail, gin.Params{{Key: "orderNo", Value: orderNo}})
	if w.Code != http.StatusForbidden {
		t.Fatalf("他人订单详情应 403: %d", w.Code)
	}

	// 原型取消
	w = invokeParams(t, "POST", "/api/orders/:orderNo/cancel", "", claims, BuyerApi{g}.Cancel, gin.Params{{Key: "orderNo", Value: orderNo}})
	if w.Code != http.StatusOK {
		t.Fatalf("原型取消失败: %d", w.Code)
	}
	w = invokeParams(t, "POST", "/api/orders/:orderNo/cancel", "", claims, BuyerApi{g}.Cancel, gin.Params{{Key: "orderNo", Value: orderNo}})
	if w.Code < 400 {
		t.Fatalf("重复取消应拒绝: %d", w.Code)
	}
	w = invokeParams(t, "POST", "/api/orders/:orderNo/cancel", "", other, BuyerApi{g}.Cancel, gin.Params{{Key: "orderNo", Value: orderNo}})
	if w.Code != http.StatusForbidden {
		t.Fatalf("他人取消应 403: %d", w.Code)
	}

	// 原型支付通知（成功 / 重复 409 / 验签失败拒绝）
	order2, err := g.OrderService.Create(buyer, pid, 1)
	if err != nil {
		t.Fatal(err)
	}
	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, order2.OrderNo, "TXN-API-1", order2.AmountCents)
	body := `{"orderNo":"` + order2.OrderNo + `","channelTxnId":"TXN-API-1","amountCents":` +
		strconv.FormatInt(order2.AmountCents, 10) + `,"sign":"` + sign + `"}`
	w = invoke(t, "POST", "/api/pay/notify", body, nil, PayApi{g}.Notify)
	if w.Code != http.StatusOK {
		t.Fatalf("原型通知失败: %d %s", w.Code, w.Body.String())
	}
	w = invoke(t, "POST", "/api/pay/notify", body, nil, PayApi{g}.Notify)
	if w.Code != http.StatusConflict {
		t.Fatalf("重复通知应 409: %d", w.Code)
	}
	w = invoke(t, "POST", "/api/pay/notify", `{"orderNo":"x","channelTxnId":"y","amountCents":1,"sign":"bad"}`, nil, PayApi{g}.Notify)
	if w.Code < 400 {
		t.Fatalf("验签失败应拒绝: %d", w.Code)
	}

	// 原型运营汇总与退款列表
	opsClaims := &utils.Claims{UserID: userIDOf(t, g, "ops1"), Role: "ops"}
	w = invoke(t, "GET", "/api/ops/summary", "", opsClaims, StaffApi{g}.Summary)
	if w.Code != http.StatusOK {
		t.Fatalf("原型汇总失败: %d", w.Code)
	}
	w = invoke(t, "GET", "/api/cs/refunds", "", opsClaims, StaffApi{g}.Refunds)
	if w.Code != http.StatusOK {
		t.Fatalf("原型退款列表失败: %d", w.Code)
	}

	// 原型渠道模式切换（SimulatedChannel 支持；非法入参拒绝）
	w = invoke(t, "POST", "/api/cs/channel/mode", `{"refundMode":"success"}`, opsClaims, PayApi{g}.ChannelMode)
	if w.Code != http.StatusOK {
		t.Fatalf("原型渠道模式切换失败: %d", w.Code)
	}
	w = invoke(t, "POST", "/api/cs/channel/mode", `{bad json`, opsClaims, PayApi{g}.ChannelMode)
	if w.Code < 400 {
		t.Fatalf("非法入参应拒绝: %d", w.Code)
	}
}
