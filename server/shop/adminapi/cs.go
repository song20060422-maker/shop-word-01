package adminapi

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/flipped-aurora/gin-vue-admin/server/model/common/response"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/model/request"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/service"
)

// ---------- 客服（GVA 管理端） ----------

// Refunds 退款单列表
func (a *Api) Refunds(c *gin.Context) {
	list, err := a.s.RefundService.List(c.Query("status"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// RefundAudit 审核售前退款（同意→渠道退款；拒绝→回到待发货）
func (a *Api) RefundAudit(c *gin.Context) {
	var req request.RefundAudit
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	actorID, _ := actor(c, "cs")
	refund, err := a.s.RefundService.Audit(actorID, req.OrderNo, req.Approve, req.Reason)
	if err != nil {
		// 审核已生效但渠道失败时，把退款单一起返回，方便客服重试
		response.FailWithDetailed(refund, service.BizCodeMsg(err), c)
		return
	}
	ok(c, refund)
}

// RefundRetry 渠道失败后重试
func (a *Api) RefundRetry(c *gin.Context) {
	var req request.RefundRetry
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	actorID, _ := actor(c, "cs")
	refund, err := a.s.RefundService.Retry(actorID, req.RefundNo)
	if err != nil {
		response.FailWithDetailed(refund, service.BizCodeMsg(err), c)
		return
	}
	ok(c, refund)
}

// Abnormals 异常收款列表（客服）
func (a *Api) CsAbnormals(c *gin.Context) {
	list, err := a.s.PaymentService.AbnormalList(c.Query("status"))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// HandleAbnormal 处理迟到支付：核实后发起退款
func (a *Api) HandleAbnormal(c *gin.Context) {
	var req request.AbnormalHandle
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	actorID, _ := actor(c, "cs")
	refund, err := a.s.RefundService.HandleAbnormal(actorID, req.ChannelTxnID, req.Reason)
	if err != nil {
		response.FailWithDetailed(refund, service.BizCodeMsg(err), c)
		return
	}
	ok(c, refund)
}

// CsNotices 支付通知处理记录（客服）
func (a *Api) CsNotices(c *gin.Context) {
	limit := 100
	if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 {
		limit = v
	}
	list, err := a.s.PaymentService.NoticesByResult(c.Query("result"), limit)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, list)
}

// CsOrderTrace 订单全链路视图（客服视角）
func (a *Api) CsOrderTrace(c *gin.Context) {
	actorID, role := actor(c, "cs")
	detail, err := a.s.ReportService.OrderDetail(c.Param("orderNo"), actorID, role)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, detail)
}

// ChannelMode 切换模拟渠道退款结果（success | fail），用于复现「首次失败、再次成功」
func (a *Api) ChannelMode(c *gin.Context) {
	var req request.ChannelMode
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, service.ErrInvalidParam)
		return
	}
	sc, isSim := a.s.Channel.(*service.SimulatedChannel)
	if !isSim {
		fail(c, service.NewBizError(9001, "当前渠道不支持切换退款模式"))
		return
	}
	sc.SetRefundMode(req.RefundMode)
	ok(c, gin.H{"refundMode": sc.RefundMode(), "refundCalls": sc.RefundCalls()})
}
