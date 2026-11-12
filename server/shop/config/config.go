// Package config 定义 GVA 风格的分组配置，并支持 yaml + 环境变量两级覆盖。
package config

import (
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// System 服务基础配置
type System struct {
	Addr string `yaml:"addr"`
	Mode string `yaml:"mode"` // debug | release | test
}

// Mysql 数据库配置
type Mysql struct {
	DSN          string `yaml:"dsn"`
	MaxIdleConns int    `yaml:"max-idle-conns"`
	MaxOpenConns int    `yaml:"max-open-conns"`
	LogMode      bool   `yaml:"log-mode"`
}

// Jwt 令牌配置
type Jwt struct {
	Secret      string `yaml:"secret"`
	ExpireHours int    `yaml:"expire-hours"`
}

// Shop 业务参数
type Shop struct {
	OrderTimeoutMinutes int    `yaml:"order-timeout-minutes"`
	PayNotifySecret     string `yaml:"pay-notify-secret"`
	DemoPassword        string `yaml:"demo-password"`
	DemoMode            bool   `yaml:"demo-mode"` // 演示数据重置开关，生产必须为 false
}

// Channel 模拟支付渠道配置
type Channel struct {
	RefundMode string `yaml:"refund-mode"` // success | fail
}

// Config 聚合配置
type Config struct {
	System  System  `yaml:"system"`
	Mysql   Mysql   `yaml:"mysql"`
	Jwt     Jwt     `yaml:"jwt"`
	Shop    Shop    `yaml:"shop"`
	Channel Channel `yaml:"channel"`
}

// Default 返回可立即运行的默认配置（本地 shop-mysql:3307）。
func Default() Config {
	return Config{
		System: System{Addr: ":8090", Mode: "debug"},
		Mysql: Mysql{
			DSN:          "root:root@tcp(127.0.0.1:3307)/shop?charset=utf8mb4&parseTime=True&loc=Local",
			MaxIdleConns: 5,
			MaxOpenConns: 20,
			LogMode:      false,
		},
		Jwt:     Jwt{Secret: "shop-gva-dev-secret", ExpireHours: 12},
		Shop:    Shop{OrderTimeoutMinutes: 10, PayNotifySecret: "channel-shared-secret", DemoPassword: "shop123"},
		Channel: Channel{RefundMode: "success"},
	}
}

// Load 读取 yaml 配置（文件不存在时使用默认值），随后应用环境变量覆盖。
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(raw, &cfg); err != nil {
				return cfg, err
			}
		} else if !os.IsNotExist(err) {
			return cfg, err
		}
	}
	if v := os.Getenv("SHOP_DSN"); v != "" {
		cfg.Mysql.DSN = v
	}
	if v := os.Getenv("SHOP_ADDR"); v != "" {
		cfg.System.Addr = v
	}
	if v := os.Getenv("SHOP_MODE"); v != "" {
		cfg.System.Mode = v
	}
	if v := os.Getenv("SHOP_JWT_SECRET"); v != "" {
		cfg.Jwt.Secret = v
	}
	if v := os.Getenv("SHOP_PAY_SECRET"); v != "" {
		cfg.Shop.PayNotifySecret = v
	}
	if v := os.Getenv("SHOP_ORDER_TIMEOUT_MINUTES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Shop.OrderTimeoutMinutes = n
		}
	}
	return cfg, nil
}

// OrderTimeout 待付款订单保留时长，缺省 10 分钟。
func (c Config) OrderTimeout() time.Duration {
	if c.Shop.OrderTimeoutMinutes <= 0 {
		return 10 * time.Minute
	}
	return time.Duration(c.Shop.OrderTimeoutMinutes) * time.Minute
}

// AllowRefundByChannel 渠道当前是否受理退款成功。
func (c Config) AllowRefundByChannel() bool { return c.Channel.RefundMode != "fail" }
