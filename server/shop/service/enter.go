// Package service 承载 Shop 交易系统的全部业务规则（类 GVA service 分层）。
// 并发互斥统一由「带前态条件的单条 UPDATE + 校验受影响行数」保证，
// 所有状态跃迁与库存变动都在数据库事务内完成。
package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"shop/shop/config"
)

// Deps 服务依赖（可注入测试库、可控时钟与假渠道）

type Deps struct {
	DB      *gorm.DB
	Now     func() time.Time
	Cfg     config.Config
	Channel ChannelClient
}

// ServiceGroup 业务服务集合（GVA GroupApp 约定）

type ServiceGroup struct {
	AuthService
	CatalogService
	OrderService
	PaymentService
	RefundService
	ShipmentService
	ReportService
	DemoService
	Channel ChannelClient
}

// NewChannelFromConfig 按 shop.pay-channel 构建渠道：alipay（配置完整时）或 nil（回落模拟渠道，默认）。
// 返回 error 仅在显式要求支付宝但配置非法时发生——启动即失败，不静默降级。

func NewChannelFromConfig(cfg config.Config) (ChannelClient, error) {
	if cfg.Shop.PayChannel != "alipay" {
		return nil, nil
	}
	return NewAlipayChannel(cfg.Shop.Alipay)
}

// NewServiceGroup 组装全部业务服务

func NewServiceGroup(d Deps) *ServiceGroup {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Channel == nil {
		d.Channel = NewSimulatedChannel(d.Cfg.AllowRefundByChannel())
	}
	g := &ServiceGroup{Channel: d.Channel}
	g.AuthService = AuthService{deps: d}
	g.CatalogService = CatalogService{deps: d}
	g.OrderService = OrderService{deps: d}
	g.PaymentService = PaymentService{deps: d}
	g.RefundService = RefundService{deps: d}
	g.ShipmentService = ShipmentService{deps: d}
	g.ReportService = ReportService{deps: d}
	g.DemoService = DemoService{deps: d}
	return g
}

// BizError 业务错误：携带稳定错误码，API 层直接映射 HTTP 状态与提示。

type BizError struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Error 实现 error 接口，msg 即对外提示文案。

func (e BizError) Error() string { return e.Msg }

// NewBizError 构造业务错误

func NewBizError(code int, msg string) BizError { return BizError{Code: code, Msg: msg} }

// 业务错误清单（对外可解释，禁止用「操作失败」这类含糊提示掩盖真实原因）

var (
	ErrInvalidParam      = NewBizError(1000, "参数不合法")
	ErrProductNotFound   = NewBizError(1001, "商品不存在")
	ErrProductOffSale    = NewBizError(1002, "商品已下架，无法购买")
	ErrInsufficientStock = NewBizError(1003, "可售数量不足")
	ErrQtyInvalid        = NewBizError(1004, "购买数量只能为 1～2 件")
	ErrReasonRequired    = NewBizError(1005, "客服处理必须填写原因")

	ErrOrderNotFound   = NewBizError(2001, "订单不存在")
	ErrForbidden       = NewBizError(2002, "无权访问他人订单")
	ErrOrderNotPayable = NewBizError(2003, "订单当前状态不允许该操作")
	ErrOrderClosed     = NewBizError(2004, "订单已关闭，不能恢复")
	ErrAlreadyPaid     = NewBizError(2005, "订单已支付，请勿重复支付")
	ErrNotPaid         = NewBizError(2006, "订单未支付，不能发货")
	ErrAlreadyShipped  = NewBizError(2007, "订单已发货，不能重复登记")
	ErrNotShippable    = NewBizError(2008, "订单当前状态不可发货")

	ErrBadSign          = NewBizError(3001, "通知验签失败")
	ErrAmountMismatch   = NewBizError(3002, "通知金额与订单金额不一致")
	ErrDuplicatedPay    = NewBizError(3003, "重复支付通知，未重复记款")
	ErrChannelRefund    = NewBizError(3004, "渠道退款失败，已保留待处理记录")
	ErrAbnormalNotFound = NewBizError(3005, "异常收款记录不存在或已处理")
	ErrAlipayDisabled   = NewBizError(3007, "支付宝渠道未启用")
	ErrTradeNotPaid     = NewBizError(3008, "交易未完成，拒绝记款")

	ErrRefundNotFound     = NewBizError(4001, "退款单不存在")
	ErrRefundExists       = NewBizError(4002, "该订单已退款或已在退款流程中")
	ErrRefundAudited      = NewBizError(4003, "退款单已审核，不能重复审核")
	ErrRefundInProgress   = NewBizError(4004, "退款处理中，暂停发货")
	ErrRefunded           = NewBizError(4005, "订单已退款，不能发货")
	ErrRefundNotRetryable = NewBizError(4006, "退款单当前状态不可重试")

	ErrLoginFailed = NewBizError(5001, "账号或密码错误")
)

// IsBiz 判断是否为业务错误（其余视为系统错误）

func IsBiz(err error) bool {
	var be BizError
	return errors.As(err, &be)
}

// BizCode 提取业务错误码

func BizCode(err error) (int, string) {
	var be BizError
	if errors.As(err, &be) {
		return be.Code, be.Msg
	}
	return 9000, "系统繁忙，请稍后重试"
}

// BizCodeMsg 仅取提示文案（接口层回传业务失败时使用）

func BizCodeMsg(err error) string {
	_, msg := BizCode(err)
	return msg
}
