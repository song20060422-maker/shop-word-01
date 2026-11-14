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

	// 内置页面（随服务发布，无需前端构建）：
	//   /          集成版全功能页：买家 / 模拟支付渠道 / 仓库 / 客服 / 运营 五个页签同页切换
	//   /shop      买家端 + 模拟支付渠道页（独立页）
	//   /ops /warehouse /cs  后台管理独立页（SHOP·OS 原型深色风格），
	//       页面经本服务 /gva/* 反向代理调用 GVA 管理端接口（gva 登录 + Casbin）
	// 运营/仓库/客服后台同时仍由 GVA 管理端（Vue 三页）提供，双入口并存。
	page := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) {
			var (
				data []byte
				err  error
			)
			switch name {
			case "all":
				data, err = web.Index()
			case "buyer":
				data, err = web.Buyer()
			case "ops":
				data, err = web.Ops()
			case "warehouse":
				data, err = web.Warehouse()
			case "cs":
				data, err = web.CS()
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
	// 根路径：集成版全功能页（买家/渠道/仓库/客服/运营 五页签同页），
	// 适合"一个页面看全部"；独立页 /shop /ops /warehouse /cs 同时保留。
	group.GET("/", page("all"))
	group.GET("/index.html", page("all"))
	group.GET("/shop", page("buyer"))
	group.GET("/shop/", page("buyer"))
	group.GET("/ops", page("ops"))
	group.GET("/ops/", page("ops"))
	group.GET("/warehouse", page("warehouse"))
	group.GET("/warehouse/", page("warehouse"))
	group.GET("/cs", page("cs"))
	group.GET("/cs/", page("cs"))

	// /gva/* 反向代理到 GVA 管理端：后台独立页面同源调用 /gva/base/login、
	// /gva/shop/** 等，浏览器无跨域；目标地址取 system.admin-target。
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
