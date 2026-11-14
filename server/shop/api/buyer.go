package api

import (
	"github.com/gin-gonic/gin"

	"shop/shop/middleware"
	"shop/shop/model"
	"shop/shop/model/request"
	"shop/shop/service"
	"shop/shop/utils"
)

// BuyerApi 买家侧接口：浏览、下单、查询、取消、售前退款

type BuyerApi struct {
	s *service.ServiceGroup
}

// Products 在售商品（含可售数量与占用情况）

func (a BuyerApi) Products(c *gin.Context) {
	list, err := a.s.CatalogService.OnSale()
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, p := range list {
		out = append(out, gin.H{
			"id":         p.ID,
			"sku":        p.SKU,
			"name":       p.Name,
			"priceCents": p.PriceCents,
			"priceText":  utils.Fen(p.PriceCents),
			"available":  p.Available,
			"onSale":     p.OnSale,
		})
	}
	ok(c, out)
}

// CreateOrder 下单：数量 1～2 件，成功即可售数量已反映占用

func (a BuyerApi) CreateOrder(c *gin.Context) {
	var req request.CreateOrder
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	claims := middleware.ClaimsOf(c)
	order, err := a.s.OrderService.Create(claims.UserID, req.ProductID, req.Qty)
	if err != nil {
		fail(c, err)
		return
	}
	available, _ := a.s.CatalogService.CurrentAvailable(req.ProductID)
	ok(c, gin.H{"order": order, "availableAfter": available, "amountText": utils.Fen(order.AmountCents)})
}

// MyOrders 我的订单

func (a BuyerApi) MyOrders(c *gin.Context) {
	claims := middleware.ClaimsOf(c)
	list, err := a.s.OrderService.ListByBuyer(claims.UserID, c.Query("status"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// OrderDetail 订单详情：可见收款、退款、发货、通知与操作轨迹

func (a BuyerApi) OrderDetail(c *gin.Context) {
	claims := middleware.ClaimsOf(c)
	detail, err := a.s.ReportService.OrderDetail(c.Param("orderNo"), claims.UserID, claims.Role)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, detail)
}

// Cancel 取消待付款订单并释放占用

func (a BuyerApi) Cancel(c *gin.Context) {
	claims := middleware.ClaimsOf(c)
	order, err := a.s.OrderService.Cancel(claims.UserID, c.Param("orderNo"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, order)
}

// RefundApply 售前退款申请（已付款未发货）

func (a BuyerApi) RefundApply(c *gin.Context) {
	var req request.RefundApply
	if err := c.ShouldBindJSON(&req); err != nil || req.OrderNo == "" {
		req.OrderNo = c.Param("orderNo")
	}
	claims := middleware.ClaimsOf(c)
	refund, err := a.s.RefundService.Apply(claims.UserID, req.OrderNo, req.Reason)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"refund": refund, "refundPending": refund.AuditStatus == model.AuditPending})
}

// MyRefund 查询本人订单退款结果

func (a BuyerApi) MyRefund(c *gin.Context) {
	claims := middleware.ClaimsOf(c)
	orderNo := c.Param("orderNo")
	order, err := a.s.OrderService.Get(orderNo)
	if err != nil {
		fail(c, err)
		return
	}
	if order.BuyerID != claims.UserID {
		fail(c, service.ErrForbidden)
		return
	}
	refund, err := a.s.RefundService.Get(orderNo)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, refund)
}
