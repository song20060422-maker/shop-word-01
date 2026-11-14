package config

import (
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// FromViper 从 GVA 全局 viper 读取 shop.* 配置段，未配置项回落到默认值。
// 这样 Shop 业务与 GVA 自身的 mysql/jwt/system 配置解耦，互不覆盖。

func FromViper(vp *viper.Viper) Config {
	cfg := Default()
	if vp == nil {
		return cfg
	}
	if v := vp.GetString("shop.mysql-dsn"); strings.TrimSpace(v) != "" {
		cfg.Mysql.DSN = v
	}
	if v := vp.GetString("shop.jwt-secret"); strings.TrimSpace(v) != "" {
		cfg.Jwt.Secret = v
	}
	if v := vp.GetString("shop.pay-notify-secret"); strings.TrimSpace(v) != "" {
		cfg.Shop.PayNotifySecret = v
	}
	if v := vp.GetString("shop.demo-password"); strings.TrimSpace(v) != "" {
		cfg.Shop.DemoPassword = v
	}
	if v := vp.GetString("shop.refund-mode"); strings.TrimSpace(v) != "" {
		cfg.Channel.RefundMode = v
	}
	if n := vp.GetInt("shop.order-timeout-minutes"); n > 0 {
		cfg.Shop.OrderTimeoutMinutes = n
	}
	if n := vp.GetInt("shop.jwt-expire-hours"); n > 0 {
		cfg.Jwt.ExpireHours = n
	}
	if v := vp.GetString("shop.mode"); strings.TrimSpace(v) != "" {
		cfg.System.Mode = v
	}
	cfg.Shop.DemoMode = vp.GetBool("shop.demo-mode")
	if v := strconv.Itoa(vp.GetInt("shop.max-open-conns")); v != "0" {
		cfg.Mysql.MaxOpenConns = vp.GetInt("shop.max-open-conns")
	}
	return cfg
}
