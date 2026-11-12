package initialize

import (
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/router"
	shopinit "github.com/flipped-aurora/gin-vue-admin/server/shop/initialize"
	shoprouter "github.com/flipped-aurora/gin-vue-admin/server/shop/router"
	"github.com/gin-gonic/gin"
)

// 占位方法，保证文件可以正确加载，避免go空变量检测报错，请勿删除。
func holder(routers ...*gin.RouterGroup) {
	_ = routers
	_ = router.RouterGroupApp
}

// initBizRouter 是 GVA 约定的业务路由扩展点，Shop 交易系统在此完成初始化与挂载。
func initBizRouter(routers ...*gin.RouterGroup) {
	privateGroup := routers[0]
	publicGroup := routers[1]

	holder(publicGroup, privateGroup)

	group, cfg, err := shopinit.Boot()
	if err != nil {
		if global.GVA_LOG != nil {
			global.GVA_LOG.Error("shop 业务模块初始化失败: " + err.Error())
		}
		return
	}
	shoprouter.InitShopRouter(publicGroup, group, cfg, time.Now)
	if global.GVA_LOG != nil {
		global.GVA_LOG.Info("shop 业务路由注册完成")
	}
}
