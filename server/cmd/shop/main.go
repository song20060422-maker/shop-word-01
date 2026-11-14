// 独立 Gin 服务：Shop 商城买家端 + 模拟支付渠道（非后台管理页面）。
//
// 与 GVA 管理端（server/main.go，默认 8099）共用同一个 gva 数据库：
//   - 本服务承载买家与渠道侧页面/接口（含接口契约路径，供验收脚本使用）
//   - 运营/仓库/客服后台页面与接口由 GVA 管理端提供（gva 登录 + Casbin）
//
// 启动：go run ./cmd/shop -c config.shop.yaml
package main

import (
	"flag"
	"log"
	"time"

	"github.com/gin-gonic/gin"

	shopconfig "shop/shop/config"
	shopdb "shop/shop/db"
	"shop/shop/migrations"
	shoprouter "shop/shop/router"
	"shop/shop/service"
)

func main() {
	cfgPath := flag.String("c", "", "配置文件路径（缺省用内置默认值，可用 SHOP_DSN/SHOP_ADDR/SHOP_MODE 等环境变量覆盖）")
	flag.Parse()

	cfg, err := shopconfig.Load(*cfgPath)
	if err != nil {
		log.Fatalf("读取配置失败: %v", err)
	}

	db, err := shopdb.Gorm(cfg)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	if err := shopdb.Migrate(db, migrations.FS); err != nil {
		log.Fatalf("执行迁移失败: %v", err)
	}
	if err := shopdb.SeedAccounts(db, cfg.Shop.DemoPassword); err != nil {
		log.Fatalf("种子账号失败: %v", err)
	}

	// 渠道装配：pay-channel=alipay 且配置完整时启用支付宝沙箱渠道，否则保持模拟渠道（默认）
	ch, err := service.NewChannelFromConfig(cfg)
	if err != nil {
		log.Fatalf("支付宝渠道初始化失败: %v", err)
	}
	g := service.NewServiceGroup(service.Deps{DB: db, Now: time.Now, Cfg: cfg, Channel: ch})
	if _, isAli := g.Channel.(*service.AlipayChannel); isAli {
		log.Printf("支付渠道：支付宝沙箱（%s），异步通知 %s", cfg.Shop.Alipay.AppID, cfg.Shop.Alipay.NotifyURL)
	} else {
		log.Print("支付渠道：模拟渠道（shop.pay-channel=alipay 可切换）")
	}

	if cfg.System.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	shoprouter.InitShopRouter(r.Group(""), g, cfg, time.Now)

	log.Printf("Shop 买家端独立服务启动：http://127.0.0.1%s/shop", cfg.System.Addr)
	if err := r.Run(cfg.System.Addr); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
