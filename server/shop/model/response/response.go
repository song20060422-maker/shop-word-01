// Package response 存放统一响应体与对外 DTO。
package response

import "github.com/flipped-aurora/gin-vue-admin/server/shop/model"

// Body 统一响应体（GVA 约定 code=0 成功）
type Body struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

// OK 成功响应
func OK(data interface{}) Body { return Body{Code: 0, Data: data, Msg: "success"} }

// Fail 失败响应
func Fail(code int, msg string) Body { return Body{Code: code, Data: nil, Msg: msg} }

// LoginResult 登录结果
type LoginResult struct {
	Token    string      `json:"token"`
	User     model.User  `json:"user"`
	ExpireAt string      `json:"expireAt"`
}

// OrderDetail 订单聚合视图：买家能看懂钱与订单去向
type OrderDetail struct {
	Order         model.Order           `json:"order"`
	Payment       *model.Payment        `json:"payment,omitempty"`
	Refund        *model.Refund         `json:"refund,omitempty"`
	Shipment      *model.Shipment       `json:"shipment,omitempty"`
	Abnormal      []model.AbnormalPayment `json:"abnormal,omitempty"`
	Notices       []model.PaymentNotice `json:"notices,omitempty"`
	Timeline      []model.OperationLog  `json:"timeline,omitempty"`
	ProductInfo   *model.Product        `json:"product,omitempty"`
	ShipableNow   bool                  `json:"shipableNow"`
	RefundPending bool                  `json:"refundPending"`
}

// ProductStat 商品维度汇总
type ProductStat struct {
	ProductID    uint64 `json:"productId"`
	SKU          string `json:"sku"`
	Name         string `json:"name"`
	PriceCents   int64  `json:"priceCents"`
	Available    int    `json:"available"`
	InitialStock int    `json:"initialStock"`
	OnSale       bool   `json:"onSale"`
	Occupied     int    `json:"occupied"`
	ShippedQty   int    `json:"shippedQty"`
	PaidCents    int64  `json:"paidCents"`
	RefundedCents int64 `json:"refundedCents"`
}

// TradeSummary 交易汇总（运营核查用）
type TradeSummary struct {
	Products          []ProductStat `json:"products"`
	TotalPaidCents    int64         `json:"totalPaidCents"`
	TotalRefundedCents int64        `json:"totalRefundedCents"`
	OrderCount        int64         `json:"orderCount"`
	StatusCount       map[string]int64 `json:"statusCount"`
}
