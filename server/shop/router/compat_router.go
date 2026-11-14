package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"shop/shop/api"
	"shop/shop/config"
	"shop/shop/middleware"
	"shop/shop/model"
	"shop/shop/model/response"
)

// InitCompatRouter 注册接口契约（api-contract.md）中新增的调用路径。
//
// 与原型路径重名的接口（/api/login、/api/products、/api/pay/notify、/api/orders 等）
// 不在本文件重复注册，改由 api.CompatApi 的 PublicGuard / AuthGuard 在路由链上委派，
// 以避免 gin 路由重复注册冲突，同时保证原型页面仍能读 data 副本。

func InitCompatRouter(pub, authed *gin.RouterGroup, ca *api.CompatApi, cfg config.Config) {
	// 测试钩子（渠道模拟器开关、强制过期）仅演示模式可用：
	// 生产环境（demo-mode=false）一律 403，防止外部操控退款结果或强改订单时效。
	demoOnly := func(handler gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if !cfg.Shop.DemoMode {
				c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(2002, "演示模式未开启，禁止调用测试钩子"))
				return
			}
			handler(c)
		}
	}
	// 公开区：模拟渠道受理付款、设定下一次渠道退款结果、超时关单测试钩子
	pub.POST("/pay/channel-pay", ca.PayChannelPay)
	pub.POST("/pay/channel-mode", demoOnly(ca.PayChannelMode))
	pub.POST("/test/expire-orders", demoOnly(ca.TestExpireOrders))

	// 买家：退款申请（契约独立路径，与原型 /orders/:orderNo/refund 并存）
	authed.POST("/orders/:orderNo/refund-apply", ca.RefundApply)

	// 运营：按 SKU/主键覆盖商品（含直接覆盖 available）、支付记录查询
	// （/api/ops/summary 已在 router.go 注册原型路由，由 api.CompatApi.AuthGuard 委派给 OpsSummary）
	ops := authed.Group("/ops", middleware.RequireRole(model.RoleOps))
	ops.PATCH("/products/:id", ca.OpsProductUpdate)
	ops.GET("/payments", ca.OpsPayments)

	// 仓库：待发货列表、发货登记
	wh := authed.Group("/warehouse", middleware.RequireRole(model.RoleWarehouse))
	wh.GET("/orders", ca.WarehouseOrders)
	wh.POST("/orders/:orderNo/ship", ca.WarehouseShip)

	// 客服：退款审核（按退款单号）、退款重试、异常支付与异常退款
	// （/api/cs/refunds 已在 router.go 注册原型路由，由 api.CompatApi.AuthGuard 委派给 CSRefunds）
	cs := authed.Group("/cs", middleware.RequireRole(model.RoleCS))
	cs.GET("/payment-exceptions", ca.CSExceptions)
	cs.POST("/payment-exceptions/:id/refund", ca.CSExceptionRefund)
	cs.POST("/refunds/:id/decide", ca.CSRefundDecide)
	cs.POST("/refunds/:id/retry", ca.CSRefundRetry)
}
