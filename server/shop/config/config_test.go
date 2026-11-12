package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestDefaultAndTimeout(t *testing.T) {
	cfg := Default()
	if cfg.Mysql.DSN == "" || cfg.Jwt.Secret == "" || cfg.Shop.PayNotifySecret == "" {
		t.Fatalf("默认配置缺少关键项: %+v", cfg)
	}
	if cfg.OrderTimeout() != 10*time.Minute {
		t.Fatalf("默认超时应为 10 分钟: %s", cfg.OrderTimeout())
	}
	cfg.Shop.OrderTimeoutMinutes = 3
	if cfg.OrderTimeout() != 3*time.Minute {
		t.Fatalf("自定义超时错误: %s", cfg.OrderTimeout())
	}
	cfg.Channel.RefundMode = "fail"
	if cfg.AllowRefundByChannel() {
		t.Fatal("渠道 fail 模式不应受理退款")
	}
	cfg.Channel.RefundMode = "success"
	if !cfg.AllowRefundByChannel() {
		t.Fatal("渠道 success 模式应受理退款")
	}
}

func TestLoadFromFileAndEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shop.yaml")
	content := `
system:
  addr: ":9099"
  mode: release
mysql:
  dsn: "root:root@tcp(127.0.0.1:3307)/shop_yaml?charset=utf8mb4"
jwt:
  secret: "yaml-secret"
  expire-hours: 6
shop:
  order-timeout-minutes: 2
  pay-notify-secret: "yaml-pay"
  demo-password: "yaml-pass"
  demo-mode: true
channel:
  refund-mode: fail
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("读取配置失败: %v", err)
	}
	if cfg.System.Addr != ":9099" || cfg.Jwt.Secret != "yaml-secret" || !cfg.Shop.DemoMode {
		t.Fatalf("配置文件未生效: %+v", cfg)
	}
	if cfg.AllowRefundByChannel() {
		t.Fatal("配置渠道失败模式未生效")
	}

	// 环境变量覆盖
	t.Setenv("SHOP_DSN", "root:root@tcp(127.0.0.1:3307)/shop_env")
	t.Setenv("SHOP_ADDR", ":9191")
	t.Setenv("SHOP_MODE", "test")
	t.Setenv("SHOP_JWT_SECRET", "env-secret")
	t.Setenv("SHOP_PAY_SECRET", "env-pay")
	t.Setenv("SHOP_ORDER_TIMEOUT_MINUTES", "5")
	cfg, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.System.Addr != ":9191" || cfg.System.Mode != "test" ||
		cfg.Jwt.Secret != "env-secret" || cfg.Shop.PayNotifySecret != "env-pay" ||
		cfg.Shop.OrderTimeoutMinutes != 5 || cfg.Mysql.DSN != "root:root@tcp(127.0.0.1:3307)/shop_env" {
		t.Fatalf("环境变量覆盖未生效: %+v", cfg)
	}

	// 非法 yaml
	badPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badPath, []byte("system: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(badPath); err == nil {
		t.Fatal("非法 yaml 应报错")
	}

	// 文件不存在回落到默认值（先清空上面设置的环境变量）
	t.Setenv("SHOP_ADDR", "")
	t.Setenv("SHOP_DSN", "")
	cfg, err = Load(filepath.Join(dir, "missing.yaml"))
	if err != nil || cfg.System.Addr != ":8090" {
		t.Fatalf("缺省配置错误: %v %+v", err, cfg)
	}
	// 空路径直接使用默认值
	if _, err := Load(""); err != nil {
		t.Fatalf("空路径不应报错: %v", err)
	}
}

func TestFromViper(t *testing.T) {
	if cfg := FromViper(nil); cfg.Jwt.Secret != Default().Jwt.Secret {
		t.Fatal("nil viper 应返回默认配置")
	}
	vp := viper.New()
	vp.Set("shop.mysql-dsn", "root:root@tcp(127.0.0.1:3307)/shop_vp")
	vp.Set("shop.jwt-secret", "vp-secret")
	vp.Set("shop.pay-notify-secret", "vp-pay")
	vp.Set("shop.demo-password", "vp-pass")
	vp.Set("shop.refund-mode", "fail")
	vp.Set("shop.order-timeout-minutes", 7)
	vp.Set("shop.jwt-expire-hours", 8)
	vp.Set("shop.mode", "release")
	vp.Set("shop.demo-mode", true)
	vp.Set("shop.max-open-conns", 30)
	cfg := FromViper(vp)
	if cfg.Mysql.DSN != "root:root@tcp(127.0.0.1:3307)/shop_vp" ||
		cfg.Jwt.Secret != "vp-secret" || cfg.Shop.PayNotifySecret != "vp-pay" ||
		cfg.Shop.DemoPassword != "vp-pass" || cfg.Channel.RefundMode != "fail" ||
		cfg.Shop.OrderTimeoutMinutes != 7 || cfg.Jwt.ExpireHours != 8 ||
		cfg.System.Mode != "release" || !cfg.Shop.DemoMode || cfg.Mysql.MaxOpenConns != 30 {
		t.Fatalf("viper 配置映射错误: %+v", cfg)
	}
}
