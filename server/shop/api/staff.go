package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"shop/shop/middleware"
	"shop/shop/model/request"
	"shop/shop/service"
)

// StaffApi 员工侧接口：运营核查、仓库发货、客服退款与异常处理

type StaffApi struct {
	s *service.ServiceGroup
}

// ---------- 运营 ----------

// ProductUpdate 上下架与调价（不影响已有订单金额快照）

func (a StaffApi) ProductUpdate(c *gin.Context) {
	var req request.ProductUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	claims := middleware.ClaimsOf(c)
	p, err := a.s.CatalogService.Update(claims.UserID, claims.Role, req.ProductID, req.OnSale, req.PriceCents)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, p)
}

// Summary 交易汇总：可售数量、已收款、已退款与状态分布

func (a StaffApi) Summary(c *gin.Context) {
	sum, err := a.s.ReportService.Summary()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, sum)
}

// Orders 按状态查询全量订单

func (a StaffApi) Orders(c *gin.Context) {
	list, err := a.s.OrderService.ListByStatus(c.Query("status"), 0)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// OrderTrace 订单全链路视图（运营/客服/仓库共用）

func (a StaffApi) OrderTrace(c *gin.Context) {
	claims := middleware.ClaimsOf(c)
	detail, err := a.s.ReportService.OrderDetail(c.Param("orderNo"), claims.UserID, claims.Role)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, detail)
}

// Notices 支付通知处理结果查询（按 result 过滤）

func (a StaffApi) Notices(c *gin.Context) {
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

func (a StaffApi) Abnormals(c *gin.Context) {
	list, err := a.s.PaymentService.AbnormalList(c.Query("status"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// Ledger 库存流水（核对占用/释放/恢复守恒）

func (a StaffApi) Ledger(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Query("productId"), 10, 64)
	list, err := a.s.ReportService.Ledger(id)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// Trail 操作留痕

func (a StaffApi) Trail(c *gin.Context) {
	list, err := a.s.ReportService.Trail(c.Query("orderNo"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// ---------- 仓库 ----------

// Shipable 待发货列表

func (a StaffApi) Shipable(c *gin.Context) {
	list, err := a.s.ShipmentService.Shipable()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// Ship 发货登记

func (a StaffApi) Ship(c *gin.Context) {
	var req request.Ship
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	claims := middleware.ClaimsOf(c)
	order, err := a.s.ShipmentService.Ship(claims.UserID, req.OrderNo, req.Carrier, req.TrackingNo)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, order)
}

// Shipments 发货记录

func (a StaffApi) Shipments(c *gin.Context) {
	list, err := a.s.ShipmentService.List()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// ---------- 客服 ----------

// Refunds 退款单列表

func (a StaffApi) Refunds(c *gin.Context) {
	list, err := a.s.RefundService.List(c.Query("status"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// RefundAudit 审核售前退款

func (a StaffApi) RefundAudit(c *gin.Context) {
	var req request.RefundAudit
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	claims := middleware.ClaimsOf(c)
	refund, err := a.s.RefundService.Audit(claims.UserID, req.OrderNo, req.Approve, req.Reason)
	if err != nil {
		// 审核已生效但渠道失败时，把退款单一起返回，方便客服重试
		c.JSON(409, gin.H{"code": 3004, "msg": service.BizCodeMsg(err), "data": refund})
		return
	}
	ok(c, refund)
}

// RefundRetry 渠道失败后重试

func (a StaffApi) RefundRetry(c *gin.Context) {
	var req request.RefundRetry
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	claims := middleware.ClaimsOf(c)
	refund, err := a.s.RefundService.Retry(claims.UserID, req.RefundNo)
	if err != nil {
		c.JSON(409, gin.H{"code": 3004, "msg": service.BizCodeMsg(err), "data": refund})
		return
	}
	ok(c, refund)
}

// HandleAbnormal 处理迟到支付：核实后发起退款

func (a StaffApi) HandleAbnormal(c *gin.Context) {
	var req request.AbnormalHandle
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	claims := middleware.ClaimsOf(c)
	refund, err := a.s.RefundService.HandleAbnormal(claims.UserID, req.ChannelTxnID, req.Reason)
	if err != nil {
		c.JSON(409, gin.H{"code": 3004, "msg": service.BizCodeMsg(err), "data": refund})
		return
	}
	ok(c, refund)
}
