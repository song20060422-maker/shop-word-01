// Package request 存放 API 层入参结构体（类 GVA model/request 分层）。
package request

// Login 登录入参
type Login struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// CreateOrder 买家下单
type CreateOrder struct {
	ProductID uint64 `json:"productId" binding:"required"`
	Qty       int    `json:"qty" binding:"required"`
}

// PaymentNotify 模拟渠道支付通知
type PaymentNotify struct {
	OrderNo      string `json:"orderNo" binding:"required"`
	ChannelTxnID string `json:"channelTxnId" binding:"required"`
	AmountCents  int64  `json:"amountCents" binding:"required"`
	Sign         string `json:"sign" binding:"required"`
}

// RefundApply 售前退款申请
type RefundApply struct {
	OrderNo string `json:"orderNo" binding:"required"`
	Reason  string `json:"reason"`
}

// RefundAudit 客服审核
type RefundAudit struct {
	OrderNo string `json:"orderNo" binding:"required"`
	Approve bool   `json:"approve"`
	Reason  string `json:"reason"`
}

// RefundRetry 渠道失败后重试
type RefundRetry struct {
	RefundNo string `json:"refundNo" binding:"required"`
}

// Ship 仓库发货登记
type Ship struct {
	OrderNo    string `json:"orderNo" binding:"required"`
	Carrier    string `json:"carrier" binding:"required"`
	TrackingNo string `json:"trackingNo" binding:"required"`
}

// AbnormalHandle 客服处理异常支付（发起退款）
type AbnormalHandle struct {
	ChannelTxnID string `json:"channelTxnId" binding:"required"`
	Reason       string `json:"reason"`
}

// ProductUpdate 运营维护商品
type ProductUpdate struct {
	ProductID  uint64 `json:"productId" binding:"required"`
	OnSale     *bool  `json:"onSale"`
	PriceCents *int64 `json:"priceCents"`
}

// AdminProductUpdate GVA 管理端商品维护入参（按 URL 主键寻址）
type AdminProductUpdate struct {
	OnSale     *bool  `json:"onSale"`
	PriceCents *int64 `json:"priceCents"`
	Available  *int   `json:"available"`
}

// ChannelMode 模拟渠道退款模式（演示用：首次失败可切换）
type ChannelMode struct {
	RefundMode string `json:"refundMode" binding:"required"` // success | fail
}
