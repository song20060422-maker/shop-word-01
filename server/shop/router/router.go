// Package router 按 GVA 分层约定注册 Shop 业务路由，挂载在 GVA 业务路由扩展点。
package router

import (
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"shop/shop/api"
	"shop/shop/config"
	"shop/shop/middleware"
	"shop/shop/model"
	"shop/shop/model/response"
	"shop/shop/service"
	"shop/shop/web"
)

// InitShopRouter 装配买家端 Gin 服务路由：公开区、登录区、买家/运营/仓库/客服接口分区，
// 页面仅托管买家端 /shop（后台页面由 GVA 管理端 Vue 提供）。

func InitShopRouter(group *gin.RouterGroup, g *service.ServiceGroup, cfg config.Config, now func() time.Time) {
	ag := api.NewApiGroup(g)
	// 接口契约兼容层：复用同一套 service，仅做入参/出参契约适配
	ca := api.NewCompatApi(g, cfg, now)

	// PublicGuard 挂在公开区路由链最前：命中契约语义不同的同名路径时委派给兼容实现
	pub := group.Group("/api", ca.PublicGuard())
	{
		pub.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "module": "shop"}) })
		pub.POST("/login", ag.Login)
		// 账号列表：演示模式下供演示页选择账号；生产环境关闭，防止枚举账号
		pub.GET("/users", func(c *gin.Context) {
			if !cfg.Shop.DemoMode {
				c.JSON(http.StatusForbidden, response.Fail(2002, "演示模式未开启，禁止查询账号列表"))
				return
			}
			ag.Users(c)
		})
		pub.GET("/products", ag.Products)
		// 模拟渠道：待支付订单查询、支付通知回调（靠签名鉴权，不需要登录态）
		pub.GET("/pay/pending", ag.PendingOrders)
		pub.POST("/pay/notify", ag.Notify)
		pub.POST("/pay/alipay/notify", ag.AlipayNotify)
		pub.GET("/pay/channel", ag.ChannelState)
		// 超时关单测试钩子：仅演示模式可用（生产由定时任务触发）
		pub.POST("/jobs/close-expired", func(c *gin.Context) {
			if !cfg.Shop.DemoMode {
				c.JSON(http.StatusForbidden, response.Fail(2002, "演示模式未开启，禁止手动触发关单"))
				return
			}
			ag.CloseExpired(c)
		})
		// 演示数据重置：仅 shop.demo-mode=true 时可用
		pub.POST("/demo/reset", func(c *gin.Context) {
			if !cfg.Shop.DemoMode {
				c.JSON(http.StatusForbidden, response.Fail(2002, "演示模式未开启，禁止重置"))
				return
			}
			ag.DemoApi.Reset(c)
		})
	}

	// 内置页面只保留买家端（随服务发布，无需前端构建）：/shop 买家 + 模拟支付渠道页。
	// 运营/仓库/客服后台页面统一由 GVA 管理端 Vue（web/src/view/shop/*.vue）提供，
	// 接口走 GVA /shop/**（gva 登录 + Casbin），本服务不再重复内置后台页。
	page := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) {
			var (
				data []byte
				err  error
			)
			switch name {
			case "buyer":
				data, err = web.Buyer()
			default:
				err = errors.New("未知页面")
			}
			if err != nil {
				c.String(http.StatusInternalServerError, "页面加载失败")
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		}
	}
	// 根路径与 /index.html 统一跳转买家页（历史入口兼容）
	group.GET("/", func(c *gin.Context) { c.Redirect(http.StatusFound, "/shop") })
	group.GET("/index.html", func(c *gin.Context) { c.Redirect(http.StatusFound, "/shop") })
	group.GET("/shop", page("buyer"))
	group.GET("/shop/", page("buyer"))
	// 后台三页入口已迁移到 GVA 管理端：命中旧路径时提示去向，避免静默 404
	adminHint := func(page string) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.String(http.StatusNotFound,
				"后台管理页面已迁移至 GVA 管理端（Vue）：请在 http://127.0.0.1:8080 登录后打开「商城管理 → %s」；买家端仅保留 /shop", page)
		}
	}
	group.GET("/ops", adminHint("运营核查"))
	group.GET("/warehouse", adminHint("仓库发货"))
	group.GET("/cs", adminHint("客服退款"))

	// /gva/* 反向代理到 GVA 管理端（历史兼容保留）：/gva/base/login、/gva/shop/** 等
	// 可经本服务同源访问，目标地址取 system.admin-target。正式后台入口为 GVA Vue（:8080）。
	adminURL, err := url.Parse(cfg.System.AdminTarget)
	if err != nil || adminURL.Scheme == "" || adminURL.Host == "" {
		log.Printf("shop: system.admin-target 非法 %q，/gva 反向代理未启用", cfg.System.AdminTarget)
	} else {
		// 剥掉 /gva 前缀再转发：/gva/base/login -> {admin-target}/base/login
		proxy := &httputil.ReverseProxy{Director: func(req *http.Request) {
			req.URL.Scheme = adminURL.Scheme
			req.URL.Host = adminURL.Host
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/gva")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			req.Host = adminURL.Host
		}}
		group.Any("/gva/*path", func(c *gin.Context) {
			proxy.ServeHTTP(c.Writer, c.Request)
		})
	}

	// AuthGuard 必须在 JWT 之后执行：委派契约实现时需要 claims 判断越权
	authed := group.Group("/api", middleware.JWT(cfg.Jwt.Secret, now), ca.AuthGuard())
	{
		authed.GET("/me", ag.Me)

		// 买家：下单、查询、取消、售前退款（契约规定买家区仅 buyer 角色可用）
		buyer := authed.Group("", middleware.RequireRole(model.RoleBuyer))
		{
			buyer.POST("/orders", ag.CreateOrder)
			buyer.GET("/orders", ag.MyOrders)
			buyer.GET("/orders/:orderNo", ag.OrderDetail)
			buyer.POST("/orders/:orderNo/cancel", ag.Cancel)
			buyer.POST("/orders/:orderNo/refund", ag.RefundApply)
			buyer.GET("/orders/:orderNo/refund", ag.MyRefund)
			buyer.POST("/orders/:orderNo/alipay-pay", ag.AlipayPay)
		}

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
	InitCompatRouter(pub, authed, ca, cfg)
}
