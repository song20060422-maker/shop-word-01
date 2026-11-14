package adminapi

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"shop/shop/model/request"
	"shop/shop/service"
)

// ---------- 运营（GVA 管理端） ----------

// Summary 交易汇总：可售数量、已收款、已退款与状态分布

func (a *Api) Summary(c *gin.Context) {
	sum, err := a.s.ReportService.Summary()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, sum)
}

// Products 全部商品（含下架）

func (a *Api) Products(c *gin.Context) {
	list, err := a.s.CatalogService.All()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// ProductUpdate 运营维护商品：上下架/调价/直接覆盖可售数量

func (a *Api) ProductUpdate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		fail(c, service.ErrInvalidParam)
		return
	}
	var req request.AdminProductUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	actorID, role := actor(c, "ops")
	if req.Available != nil {
		p, err := a.s.CatalogService.OverwriteAvailable(actorID, role, id, *req.Available)
		if err != nil {
			fail(c, err)
			return
		}
		c.Set("shopProduct", p)
	}
	if req.OnSale != nil || req.PriceCents != nil {
		p, err := a.s.CatalogService.Update(actorID, role, id, req.OnSale, req.PriceCents)
		if err != nil {
			fail(c, err)
			return
		}
		c.Set("shopProduct", p)
	}
	pv, existed := c.Get("shopProduct")
	if !existed {
		fail(c, service.ErrInvalidParam)
		return
	}
	ok(c, pv)
}

// Orders 按状态查询全量订单

func (a *Api) Orders(c *gin.Context) {
	list, err := a.s.OrderService.ListByStatus(c.Query("status"), 0)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// OrderTrace 订单全链路核查视图

func (a *Api) OrderTrace(c *gin.Context) {
	actorID, role := actor(c, "ops")
	detail, err := a.s.ReportService.OrderDetail(c.Param("orderNo"), actorID, role)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, detail)
}

// Notices 支付通知处理结果查询（按 orderNo / result 过滤）

func (a *Api) Notices(c *gin.Context) {
	no := c.Query("orderNo")
	if no != "" {
		list, err := a.s.PaymentService.Notices(no)
		if err != nil {
			fail(c, err)
			return
		}
		ok(c, list)
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	list, err := a.s.PaymentService.NoticesByResult(c.Query("result"), limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// Abnormals 异常收款列表

func (a *Api) Abnormals(c *gin.Context) {
	list, err := a.s.PaymentService.AbnormalList(c.Query("status"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// Ledger 库存流水（核对占用/释放/恢复守恒）

func (a *Api) Ledger(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Query("productId"), 10, 64)
	list, err := a.s.ReportService.Ledger(id)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// Trail 操作留痕

func (a *Api) Trail(c *gin.Context) {
	list, err := a.s.ReportService.Trail(c.Query("orderNo"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}
