// GVA 管理端（后台）商城路由：挂在 PrivateGroup，走 GVA JWT + Casbin。
package router

import (
	"github.com/gin-gonic/gin"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/adminapi"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/service"
)

// InitShopAdminRouter 注册商城后台管理接口（运营/仓库/客服），前缀 /shop。
// 调用方需保证 group 已经过 GVA JWTAuth + CasbinHandler。
func InitShopAdminRouter(group *gin.RouterGroup, g *service.ServiceGroup) {
	a := adminapi.NewApi(g)

	ops := group.Group("/shop/ops")
	{
		ops.GET("/summary", a.Summary)
		ops.GET("/products", a.Products)
		ops.PATCH("/products/:id", a.ProductUpdate)
		ops.GET("/orders", a.Orders)
		ops.GET("/orders/:orderNo", a.OrderTrace)
		ops.GET("/notices", a.Notices)
		ops.GET("/abnormals", a.Abnormals)
		ops.GET("/ledger", a.Ledger)
		ops.GET("/trail", a.Trail)
	}

	wh := group.Group("/shop/warehouse")
	{
		wh.GET("/shipable", a.Shipable)
		wh.POST("/ship", a.Ship)
		wh.GET("/shipments", a.Shipments)
		wh.GET("/orders/:orderNo", a.WarehouseOrderTrace)
	}

	cs := group.Group("/shop/cs")
	{
		cs.GET("/refunds", a.Refunds)
		cs.POST("/refunds/audit", a.RefundAudit)
		cs.POST("/refunds/retry", a.RefundRetry)
		cs.GET("/abnormals", a.CsAbnormals)
		cs.POST("/abnormals/handle", a.HandleAbnormal)
		cs.GET("/notices", a.CsNotices)
		cs.GET("/orders/:orderNo", a.CsOrderTrace)
		cs.POST("/channel/mode", a.ChannelMode)
	}
}
