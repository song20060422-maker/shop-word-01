package initialize

import (
	"time"

	"shop/global"
	"shop/shop/config"
	shopdb "shop/shop/db"
	"shop/shop/migrations"
	"shop/shop/service"
)

// Boot 初始化 Shop 业务模块：读取 shop 配置段 -> 连接数据库 -> 执行迁移 -> 补齐种子账号。
// 返回装配完成的 ServiceGroup，供路由层直接消费。
// 注意：Shop 使用独立连接（Deps.DB 注入），严禁覆盖 global.GVA_DB——
// 它是 Gin-Vue-Admin 框架主库（gva）的连接，覆盖会导致管理后台查询 shop 库失败。

func Boot() (*service.ServiceGroup, config.Config, error) {
	cfg := config.FromViper(global.GVA_VP)

	db, err := shopdb.Gorm(cfg)
	if err != nil {
		return nil, cfg, err
	}
	if err := shopdb.Migrate(db, migrations.FS); err != nil {
		return nil, cfg, err
	}
	if err := shopdb.SeedAccounts(db, cfg.Shop.DemoPassword); err != nil {
		return nil, cfg, err
	}

	now := time.Now
	// 渠道装配：pay-channel=alipay 时管理端退款走支付宝沙箱真实渠道；失败即启动失败，不静默降级
	ch, err := service.NewChannelFromConfig(cfg)
	if err != nil {
		return nil, cfg, err
	}
	group := service.NewServiceGroup(service.Deps{DB: db, Now: now, Cfg: cfg, Channel: ch})
	if global.GVA_LOG != nil {
		if _, isAli := group.Channel.(*service.AlipayChannel); isAli {
			global.GVA_LOG.Info("shop 业务模块初始化完成（支付渠道：支付宝沙箱）")
		} else {
			global.GVA_LOG.Info("shop 业务模块初始化完成（支付渠道：模拟渠道）")
		}
	}
	return group, cfg, nil
}
