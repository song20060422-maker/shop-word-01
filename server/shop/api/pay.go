package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"shop/shop/model/request"
	"shop/shop/model/response"
	"shop/shop/service"
)

// PayApi 模拟支付渠道与系统侧补偿接口

type PayApi struct {
	s *service.ServiceGroup
}

// Notify 渠道支付成功通知：不依赖登录态，靠签名鉴权；任何分支都留痕可查。

func (a PayApi) Notify(c *gin.Context) {
	raw, _ := c.GetRawData()
	var req request.PaymentNotify
	_ = json.Unmarshal(raw, &req)

	res, err := a.s.PaymentService.Notify(req.OrderNo, req.ChannelTxnID, req.AmountCents, req.Sign, string(raw))
	code, msg := 0, "success"
	status := http.StatusOK
	if err != nil {
		code, msg = service.BizCode(err)
		status = statusOf(code)
	}
	c.JSON(status, response.Body{Code: code, Data: res, Msg: msg})
}

// CloseExpired 服务端时间触发的超时关闭（演示可手动触发，也可由定时任务调用）

func (a PayApi) CloseExpired(c *gin.Context) {
	n, err := a.s.OrderService.CloseExpired()
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"closed": n})
}

// ChannelMode 切换模拟渠道退款结果（success | fail），用于复现「首次失败、再次成功」

func (a PayApi) ChannelMode(c *gin.Context) {
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

// PendingOrders 模拟渠道侧可见的待支付订单（演示页面据此发起支付/重发/迟到通知）

func (a PayApi) PendingOrders(c *gin.Context) {
	list, err := a.s.OrderService.ListByStatus("pending_payment", 50)
	if err != nil {
		fail(c, err)
		return
	}
	out := make([]gin.H, 0, len(list))
	for _, o := range list {
		out = append(out, gin.H{
			"orderNo":     o.OrderNo,
			"productName": o.ProductName,
			"amountCents": o.AmountCents,
			"expireAt":    o.ExpireAt,
			"createdAt":   o.CreatedAt,
		})
	}
	ok(c, out)
}

// ChannelState 查看渠道当前模式（演示与验收脚本用）

func (a PayApi) ChannelState(c *gin.Context) {
	sc, isSim := a.s.Channel.(*service.SimulatedChannel)
	if !isSim {
		ok(c, gin.H{"refundMode": "external"})
		return
	}
	ok(c, gin.H{"refundMode": sc.RefundMode(), "refundCalls": sc.RefundCalls()})
}
