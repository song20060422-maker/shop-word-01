package initialize

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"shop/global"
	shopconfig "shop/shop/config"
	shopdb "shop/shop/db"
)

// 共享测试库（shop_test）中可能残留其他包或历史运行的数据，
// Boot 依赖迁移+种子+密码补齐，测试前先按 FK 安全顺序清空业务表。
func resetTables(t *testing.T, dsn string) {
	t.Helper()
	cfg := shopconfig.Default()
	cfg.Mysql.DSN = dsn
	db, err := shopdb.Gorm(cfg)
	if err != nil {
		t.Fatalf("复位连接失败: %v", err)
	}
	for _, tbl := range []string{
		"sessions", "abnormal_payments", "payments", "payment_notices", "refunds",
		"shipments", "inventory_ledger", "operation_logs", "orders", "products", "users",
	} {
		if err := db.Exec("DELETE FROM " + tbl).Error; err != nil {
			t.Fatalf("复位 %s 失败: %v", tbl, err)
		}
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// Boot 测试：从 viper 读取 shop.* 配置段，连库 -> 迁移 -> 种子 -> 装配 ServiceGroup。
// 数据库可用 SHOP_TEST_DSN 覆盖（默认 shop_test，须 -p 1 串行执行）。
func TestBoot(t *testing.T) {
	dsn := os.Getenv("SHOP_TEST_DSN")
	if dsn == "" {
		dsn = "root:root@tcp(127.0.0.1:3307)/shop_test?charset=utf8mb4&parseTime=True&loc=Local"
	}
	resetTables(t, dsn)
	vp := viper.New()
	vp.Set("shop.mysql-dsn", dsn)
	vp.Set("shop.jwt-secret", "boot-secret")
	vp.Set("shop.pay-notify-secret", "boot-pay")
	vp.Set("shop.demo-password", "shop123")
	vp.Set("shop.order-timeout-minutes", 10)
	global.GVA_VP = vp
	global.GVA_LOG = zap.NewNop()

	group, cfg, err := Boot()
	if err != nil {
		t.Fatalf("Boot 失败: %v", err)
	}
	if group == nil || cfg.Jwt.Secret != "boot-secret" {
		t.Fatalf("Boot 装配结果错误: %+v", cfg)
	}
	// Shop 使用独立连接，严禁覆盖框架全局 GVA_DB（gva 主库）
	if global.GVA_DB != nil {
		t.Fatal("Boot 不应覆盖 global.GVA_DB")
	}
	// 种子账号可登录
	if _, err := group.AuthService.Login("buyer1", "shop123"); err != nil {
		t.Fatalf("种子账号登录失败: %v", err)
	}
	// 二次启动幂等（迁移与种子可重复执行）
	if _, _, err := Boot(); err != nil {
		t.Fatalf("重复 Boot 失败: %v", err)
	}
}

// 连库失败时 Boot 必须返回错误，不允许带病启动。
func TestBootFailsOnBadDSN(t *testing.T) {
	vp := viper.New()
	vp.Set("shop.mysql-dsn", "root:wrong@tcp(127.0.0.1:3399)/nope?charset=utf8mb4")
	global.GVA_VP = vp
	global.GVA_LOG = nil
	if _, _, err := Boot(); err == nil {
		t.Fatal("错误 DSN 应启动失败")
	}
}
