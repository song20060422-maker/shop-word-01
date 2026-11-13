package initialize

import (
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

// initBizRouter 是 GVA 约定的业务路由扩展点。
// 商城架构（2026-11-13 拆分）：
//   - 买家端 + 模拟支付渠道：独立 Gin 服务（server/cmd/shop，默认 :8090），
//     不再挂载到 GVA 管理端；
//   - 运营/仓库/客服后台：GVA 管理端接口（/shop/**，走 gva 登录 + Casbin）
//     与管理端 Vue 页面（web/src/view/shop/**）。
//
// 业务表统一存放在 gva 库（gva 框架表 + shop 业务 11 张表）。
func initBizRouter(routers ...*gin.RouterGroup) {
	privateGroup := routers[0]
	publicGroup := routers[1]

	holder(publicGroup, privateGroup)

	group, _, err := shopinit.Boot()
	if err != nil {
		if global.GVA_LOG != nil {
			global.GVA_LOG.Error("shop 业务模块初始化失败: " + err.Error())
		}
		return
	}
	shoprouter.InitShopAdminRouter(privateGroup, group)
	if global.GVA_LOG != nil {
		global.GVA_LOG.Info("shop 后台管理路由注册完成（/shop/**）")
	}
}
