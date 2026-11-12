package service

import (
	"os"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/config"
	shopinit "github.com/flipped-aurora/gin-vue-admin/server/shop/initialize"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/migrations"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/utils"
)

// 说明：service 层测试使用真实 MySQL（本机 3307），每个用例前清空业务数据并重放迁移/种子。
// 数据库可用 SHOP_TEST_DSN 覆盖，默认库名 shop_test_service（各测试包使用独立库，避免相互干扰）。

var testDB *gorm.DB
var testCfg config.Config

func TestMain(m *testing.M) {
	dsn := os.Getenv("SHOP_TEST_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3307)/shop_test_service?charset=utf8mb4&parseTime=True&loc=Local"
	}
	testCfg = config.Default()
	testCfg.Mysql.DSN = dsn
	testCfg.Shop.PayNotifySecret = "test-secret"
	testCfg.Shop.OrderTimeoutMinutes = 10
	db, err := shopinit.Gorm(testCfg)
	if err != nil {
		panic("连接测试库失败（请确认 MySQL 3307 可用）: " + err.Error())
	}
	if err := shopinit.Migrate(db, migrations.FS); err != nil {
		panic("测试库迁移失败: " + err.Error())
	}
	if err := shopinit.SeedAccounts(db, "shop123"); err != nil {
		panic("种子账号失败: " + err.Error())
	}
	testDB = db
	os.Exit(m.Run())
}

type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

func resetDB(t *testing.T) {
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
	if err := shopinit.Migrate(testDB, migrations.FS); err != nil {
		t.Fatalf("重放迁移失败: %v", err)
	}
	if err := shopinit.SeedAccounts(testDB, "shop123"); err != nil {
		t.Fatalf("重放种子账号失败: %v", err)
	}
}

func newEnv(t *testing.T) (*ServiceGroup, *SimulatedChannel, *fakeClock) {
	t.Helper()
	resetDB(t)
	clock := &fakeClock{t: time.Date(2026, 11, 12, 10, 0, 0, 0, time.Local)}
	ch := NewSimulatedChannel(true)
	cfg := testCfg
	g := NewServiceGroup(Deps{DB: testDB, Now: clock.Now, Cfg: cfg, Channel: ch})
	return g, ch, clock
}

func buyerID(t *testing.T, username string) uint64 {
	t.Helper()
	var id uint64
	if err := testDB.Table("users").Where("username = ?", username).Pluck("id", &id).Error; err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatalf("未找到账号 %s", username)
	}
	return id
}

func productID(t *testing.T, sku string) uint64 {
	t.Helper()
	var id uint64
	if err := testDB.Table("products").Where("sku = ?", sku).Pluck("id", &id).Error; err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatalf("未找到商品 %s", sku)
	}
	return id
}

func available(t *testing.T, sku string) int {
	t.Helper()
	var n int
	if err := testDB.Table("products").Where("sku = ?", sku).Pluck("available", &n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func payOrder(t *testing.T, g *ServiceGroup, buyer uint64, qty int) (orderNo, txn string, amount int64) {
	t.Helper()
	pid := productID(t, "SKU-A")
	o, err := g.OrderService.Create(buyer, pid, qty)
	if err != nil {
		t.Fatalf("下单失败: %v", err)
	}
	txn = "TXN-" + o.OrderNo
	sign := utils.PaySign(testCfg.Shop.PayNotifySecret, o.OrderNo, txn, o.AmountCents)
	res, err := g.PaymentService.Notify(o.OrderNo, txn, o.AmountCents, sign, "{}")
	if err != nil || res.Result != "accepted" {
		t.Fatalf("支付通知失败: %v %+v", err, res)
	}
	return o.OrderNo, txn, o.AmountCents
}
