// Package router 按 GVA 分层约定注册 Shop 业务路由，挂载在 GVA 业务路由扩展点。
package router

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/api"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/config"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/middleware"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/model"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/model/response"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/service"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/web"
)

// InitShopRouter 装配 Shop 全部路由：公开区、登录区、运营/仓库/客服分区。
func InitShopRouter(group *gin.RouterGroup, g *service.ServiceGroup, cfg config.Config, now func() time.Time) {
	ag := api.NewApiGroup(g)
	// 接口契约兼容层：复用同一套 service，仅做入参/出参契约适配
	ca := api.NewCompatApi(g, cfg, now)

	// PublicGuard 挂在公开区路由链最前：命中契约语义不同的同名路径时委派给兼容实现
	pub := group.Group("/api", ca.PublicGuard())
	{
		pub.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "module": "shop"}) })
		pub.POST("/login", ag.Login)
		pub.GET("/users", ag.Users)
		pub.GET("/products", ag.Products)
		// 模拟渠道：待支付订单查询、支付通知回调（靠签名鉴权，不需要登录态）
		pub.GET("/pay/pending", ag.PendingOrders)
		pub.POST("/pay/notify", ag.Notify)
		pub.GET("/pay/channel", ag.ChannelState)
		pub.POST("/jobs/close-expired", ag.CloseExpired)
		// 演示数据重置：仅 shop.demo-mode=true 时可用
		pub.POST("/demo/reset", func(c *gin.Context) {
			if !cfg.Shop.DemoMode {
				c.JSON(http.StatusForbidden, response.Fail(2002, "演示模式未开启，禁止重置"))
				return
			}
			ag.DemoApi.Reset(c)
		})
	}

	// 内置原型页面（买家/渠道/仓库/客服/运营），随服务发布，无需前端构建
	page := func(c *gin.Context) {
		data, err := web.Index()
		if err != nil {
			c.String(http.StatusInternalServerError, "页面加载失败")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	}
	group.GET("/shop", page)
	group.GET("/shop/", page)

	// AuthGuard 必须在 JWT 之后执行：委派契约实现时需要 claims 判断越权
	authed := group.Group("/api", middleware.JWT(cfg.Jwt.Secret, now), ca.AuthGuard())
	{
		authed.GET("/me", ag.Me)

		// 买家：下单、查询、取消、售前退款
		authed.POST("/orders", ag.CreateOrder)
		authed.GET("/orders", ag.MyOrders)
		authed.GET("/orders/:orderNo", ag.OrderDetail)
		authed.POST("/orders/:orderNo/cancel", ag.Cancel)
		authed.POST("/orders/:orderNo/refund", ag.RefundApply)
		authed.GET("/orders/:orderNo/refund", ag.MyRefund)

		// 运营：上下架、核查、查询
		ops := authed.Group("/ops", middleware.RequireRole(model.RoleOps))
		{
			ops.PATCH("/products", ag.ProductUpdate)
			ops.GET("/summary", ag.Summary)
			ops.GET("/orders", ag.Orders)
			ops.GET("/orders/:orderNo", ag.OrderTrace)
			ops.GET("/notices", ag.Notices)
			ops.GET("/abnormals", ag.Abnormals)
			ops.GET("/ledger", ag.Ledger)
			ops.GET("/trail", ag.Trail)
		}

		// 仓库：待发货与发货登记
		wh := authed.Group("/warehouse", middleware.RequireRole(model.RoleWarehouse))
		{
			wh.GET("/shipable", ag.Shipable)
			wh.POST("/ship", ag.Ship)
			wh.GET("/shipments", ag.Shipments)
			wh.GET("/orders/:orderNo", ag.OrderTrace)
		}

		// 客服：退款审核、渠道重试、异常收款处理
		cs := authed.Group("/cs", middleware.RequireRole(model.RoleCS))
		{
			cs.GET("/refunds", ag.Refunds)
			cs.POST("/refunds/audit", ag.RefundAudit)
			cs.POST("/refunds/retry", ag.RefundRetry)
			cs.GET("/abnormals", ag.Abnormals)
			cs.POST("/abnormals/handle", ag.HandleAbnormal)
			cs.GET("/notices", ag.Notices)
			cs.GET("/orders/:orderNo", ag.OrderTrace)
			cs.POST("/channel/mode", ag.ChannelMode)
		}
	}

	// 契约新增路径（不与原型重名的部分）
	InitCompatRouter(pub, authed, ca)
}
