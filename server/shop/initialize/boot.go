package initialize

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/config"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/migrations"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/service"
)

// Boot 初始化 Shop 业务模块：读取 shop 配置段 -> 连接数据库 -> 执行迁移 -> 补齐种子账号。
// 返回装配完成的 ServiceGroup，供路由层直接消费。
func Boot() (*service.ServiceGroup, config.Config, error) {
	cfg := config.FromViper(global.GVA_VP)

	db, err := Gorm(cfg)
	if err != nil {
		return nil, cfg, err
	}
	if err := Migrate(db, migrations.FS); err != nil {
		return nil, cfg, err
	}
	if err := SeedAccounts(db, cfg.Shop.DemoPassword); err != nil {
		return nil, cfg, err
	}
	global.GVA_DB = db

	now := time.Now
	group := service.NewServiceGroup(service.Deps{DB: db, Now: now, Cfg: cfg})
	if global.GVA_LOG != nil {
		global.GVA_LOG.Info("shop 业务模块初始化完成")
	}
	return group, cfg, nil
}
