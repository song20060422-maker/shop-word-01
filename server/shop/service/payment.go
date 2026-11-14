package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/smartwalle/alipay/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"shop/shop/model"
	"shop/shop/utils"
)

// isDuplicateEntry 判定是否 MySQL 唯一键冲突（errno 1062）

func isDuplicateEntry(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}

// errDuplicatePayment 内部哨兵：触发事务回滚，外层据此判定重复通知。

var errDuplicatePayment = errors.New("duplicate payment")

// NotifyResult 支付通知处理结果（渠道与运营都可查询）

type NotifyResult struct {
	Result       string `json:"result"` // accepted | rejected | duplicated | abnormal
	OrderNo      string `json:"orderNo"`
	ChannelTxnID string `json:"channelTxnId"`
	AmountCents  int64  `json:"amountCents"`
	Reason       string `json:"reason"`
	OrderStatus  string `json:"orderStatus"`
	NoticeID     uint64 `json:"noticeId"`
	AbnormalID   uint64 `json:"abnormalId,omitempty"`
}

// PaymentService 支付通知受理与异常收款处理

type PaymentService struct {
	deps Deps
}

// Notify 受理模拟渠道支付通知：HMAC 验签 → 复用 applyPayment 推进。

func (s PaymentService) Notify(orderNo, txn string, amount int64, sign, rawBody string) (NotifyResult, error) {
	now := s.deps.Now()
	orderNo = strings.TrimSpace(orderNo)
	txn = strings.TrimSpace(txn)

	// 1. 参数与验签：验签失败必须留痕，且绝不记款
	if orderNo == "" || txn == "" || amount <= 0 || sign == "" {
		result := NotifyResult{Result: model.NoticeRejected, OrderNo: orderNo, ChannelTxnID: txn, AmountCents: amount, Reason: "参数不合法"}
		s.saveNotice(result, false, rawBody, now)
		return result, ErrInvalidParam
	}
	if utils.PaySign(s.deps.Cfg.Shop.PayNotifySecret, orderNo, txn, amount) != sign {
		result := NotifyResult{Result: model.NoticeRejected, OrderNo: orderNo, ChannelTxnID: txn, AmountCents: amount, Reason: "验签失败"}
		s.saveNotice(result, false, rawBody, now)
		return result, ErrBadSign
	}
	return s.applyPayment(orderNo, txn, amount, rawBody, now)
}

// NotifyAlipay 支付宝异步通知：RSA2 验签（渠道内实现）→ app_id/交易状态校验 → 复用同一套推进逻辑，
// 保证真实渠道与模拟渠道在订单状态机、幂等与异常收款上行为完全一致。

func (s PaymentService) NotifyAlipay(form url.Values, rawBody string) (NotifyResult, error) {
	now := s.deps.Now()
	result := NotifyResult{Result: model.NoticeRejected,
		OrderNo: form.Get("out_trade_no"), ChannelTxnID: form.Get("trade_no")}
	ch, isAli := s.deps.Channel.(NotifyVerifier)
	if !isAli {
		result.Reason = "支付宝渠道未启用"
		s.saveNotice(result, false, rawBody, now)
		return result, ErrAlipayDisabled
	}
	payload, err := ch.VerifyNotify(context.Background(), form)
	result.OrderNo = payload.OrderNo
	result.ChannelTxnID = payload.Txn
	result.AmountCents = payload.AmountCents
	if err != nil {
		result.Reason = "验签失败：" + err.Error()
		s.saveNotice(result, false, rawBody, now)
		return result, ErrBadSign
	}
	if alipay.TradeStatus(payload.TradeStatus) != alipay.TradeStatusSuccess && alipay.TradeStatus(payload.TradeStatus) != alipay.TradeStatusFinished {
		result.Reason = "交易未完成：" + payload.TradeStatus
		s.saveNotice(result, false, rawBody, now)
		return result, ErrTradeNotPaid
	}
	return s.applyPayment(payload.OrderNo, payload.Txn, payload.AmountCents, rawBody, now)
}

// applyPayment 验签通过后的共同路径：匹配订单与金额 → 幂等记款 / 迟到支付转异常。

func (s PaymentService) applyPayment(orderNo, txn string, amount int64, rawBody string, now time.Time) (NotifyResult, error) {
	result := NotifyResult{OrderNo: orderNo, ChannelTxnID: txn, AmountCents: amount}

	err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
		var o model.Order
		if err := tx.Where("order_no = ?", orderNo).First(&o).Error; err != nil {
			result.Result = model.NoticeRejected
			result.Reason = "未知订单"
			return s.noticeInTx(tx, &result, true, rawBody, now)
		}
		result.OrderStatus = o.Status
		if amount != o.AmountCents {
			result.Result = model.NoticeRejected
			result.Reason = "金额不符：订单应收 " + itoa64(o.AmountCents) + " 分，通知为 " + itoa64(amount) + " 分"
			return s.noticeInTx(tx, &result, true, rawBody, now)
		}

		// 2. 订单已不在「待付款」：重复通知或迟到支付
		if o.Status != model.OrderStatusPendingPayment {
			if isClosedStatus(o.Status) {
				return s.markAbnormal(tx, &result, amount, now, rawBody, "订单超时关闭后收到支付成功通知，需客服核实退款")
			}
			result.Result = model.NoticeDuplicated
			result.Reason = "订单已" + statusLabel(o.Status) + "，通知不重复记款"
			return s.noticeInTx(tx, &result, true, rawBody, now)
		}

		// 3. 有效期内：条件推进订单状态，再记款；唯一键冲突即整体回滚
		res := tx.Model(&model.Order{}).
			Where("order_no = ? AND status = ? AND expire_at > ?", orderNo, model.OrderStatusPendingPayment, now).
			Updates(map[string]interface{}{"status": model.OrderStatusPaid, "paid_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			var latest model.Order
			if err := tx.Where("order_no = ?", orderNo).First(&latest).Error; err != nil {
				return err
			}
			if isClosedStatus(latest.Status) {
				return s.markAbnormal(tx, &result, amount, now, rawBody, "通知到达时订单已关闭，转入异常收款待处理")
			}
			result.Result = model.NoticeDuplicated
			result.Reason = "订单已" + statusLabel(latest.Status) + "，通知不重复记款"
			return s.noticeInTx(tx, &result, true, rawBody, now)
		}
		pay := model.Payment{OrderNo: orderNo, ChannelTxnID: txn, AmountCents: amount, PaidAt: now, CreatedAt: now}
		if err := tx.Create(&pay).Error; err != nil {
			// I-18 错误分类：只有唯一键冲突（重复交易号/一单多记款）才判重复支付；
			// 其它 DB 错误按真实错误上抛，不再一律误报为「重复支付」。
			if isDuplicateEntry(err) {
				return errDuplicatePayment
			}
			return err
		}
		result.Result = model.NoticeAccepted
		result.OrderStatus = model.OrderStatusPaid
		result.Reason = "记款成功，订单进入待发货"
		if err := s.noticeInTx(tx, &result, true, rawBody, now); err != nil {
			return err
		}
		return logOp(tx, &orderNo, nil, "system", "payment_accepted",
			"渠道记款："+txn+"，金额 "+itoa64(amount)+" 分")
	})
	if err != nil && errors.Is(err, errDuplicatePayment) {
		dup := NotifyResult{Result: model.NoticeDuplicated, OrderNo: orderNo, ChannelTxnID: txn,
			AmountCents: amount, Reason: "重复支付通知，未重复记款", OrderStatus: model.OrderStatusPaid}
		s.saveNotice(dup, true, rawBody, now)
		return dup, ErrDuplicatedPay
	}
	if err != nil {
		return result, err
	}
	if result.Result == model.NoticeDuplicated {
		return result, ErrDuplicatedPay
	}
	return result, nil
}

// AlipayPayURL 为待付款订单生成支付宝收银台跳转地址（电脑网站支付）：
// 校验本人订单与待付款状态，渠道未启用返回业务错误。

func (s PaymentService) AlipayPayURL(orderNo string, buyerID uint64, publicBase string) (string, error) {
	var o model.Order
	if err := s.deps.DB.Where("order_no = ?", strings.TrimSpace(orderNo)).First(&o).Error; err != nil {
		return "", ErrOrderNotFound
	}
	if o.BuyerID != buyerID {
		return "", ErrForbidden
	}
	if o.Status != model.OrderStatusPendingPayment {
		return "", ErrOrderNotPayable
	}
	ac, isAli := s.deps.Channel.(*AlipayChannel)
	if !isAli {
		return "", ErrAlipayDisabled
	}
	// 回调地址不写死：买家从哪个域名访问，通知就回哪个域名（config 显式配置仍优先生效）
	notifyURL, returnURL := "", ""
	if b := strings.TrimSpace(publicBase); b != "" {
		notifyURL = b + "/api/pay/alipay/notify"
		returnURL = b + "/shop"
	}
	return ac.PagePayURL(o.OrderNo, o.AmountCents, "商城订单："+o.ProductName, notifyURL, returnURL)
}

// markAbnormal 迟到支付：登记异常收款，绝不恢复订单、绝不重新占用库存。

func (s PaymentService) markAbnormal(tx *gorm.DB, r *NotifyResult, amount int64, now time.Time, rawBody, reason string) error {
	ab := model.AbnormalPayment{
		OrderNo:      r.OrderNo,
		ChannelTxnID: r.ChannelTxnID,
		AmountCents:  amount,
		Reason:       reason,
		Status:       model.AbnormalPending,
		CreatedAt:    now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ab).Error; err != nil {
		return err
	}
	var saved model.AbnormalPayment
	if err := tx.Where("channel_txn_id = ?", r.ChannelTxnID).First(&saved).Error; err == nil {
		r.AbnormalID = saved.ID
	}
	r.Result = model.NoticeAbnormal
	r.Reason = reason
	if err := s.noticeInTx(tx, r, true, rawBody, now); err != nil {
		return err
	}
	return logOp(tx, &r.OrderNo, nil, "system", "abnormal_payment", "迟到支付登记："+r.ChannelTxnID)
}

// noticeInTx 事务内写通知留痕

func (s PaymentService) noticeInTx(tx *gorm.DB, r *NotifyResult, signOK bool, rawBody string, now time.Time) error {
	body := rawBody
	notice := model.PaymentNotice{
		OrderNo:      r.OrderNo,
		ChannelTxnID: r.ChannelTxnID,
		AmountCents:  r.AmountCents,
		SignOK:       signOK,
		Result:       r.Result,
		Reason:       r.Reason,
		RawBody:      &body,
		CreatedAt:    now,
	}
	if err := tx.Create(&notice).Error; err != nil {
		return err
	}
	r.NoticeID = notice.ID
	return nil
}

// saveNotice 独立写通知留痕（验签失败等不进入主事务的场景）

func (s PaymentService) saveNotice(r NotifyResult, signOK bool, rawBody string, now time.Time) {
	body := rawBody
	_ = s.deps.DB.Create(&model.PaymentNotice{
		OrderNo:      r.OrderNo,
		ChannelTxnID: r.ChannelTxnID,
		AmountCents:  r.AmountCents,
		SignOK:       signOK,
		Result:       r.Result,
		Reason:       r.Reason,
		RawBody:      &body,
		CreatedAt:    now,
	}).Error
}

// Notices 按订单查询通知处理结果（需求：金额不符/未知订单/验签失败须可查询）

func (s PaymentService) Notices(orderNo string) ([]model.PaymentNotice, error) {
	var list []model.PaymentNotice
	err := s.deps.DB.Where("order_no = ?", strings.TrimSpace(orderNo)).Order("id asc").Find(&list).Error
	return list, err
}

// NoticesByResult 运营按处理结果过滤（可为空）

func (s PaymentService) NoticesByResult(result string, limit int) ([]model.PaymentNotice, error) {
	var list []model.PaymentNotice
	q := s.deps.DB
	if result != "" {
		q = q.Where("result = ?", result)
	}
	if limit <= 0 {
		limit = 100
	}
	err := q.Order("id desc").Limit(limit).Find(&list).Error
	return list, err
}

// AbnormalList 异常收款待处理列表

func (s PaymentService) AbnormalList(status string) ([]model.AbnormalPayment, error) {
	var list []model.AbnormalPayment
	q := s.deps.DB
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("id desc").Find(&list).Error
	return list, err
}

// isClosedStatus 判断订单是否已终结（关闭或取消）：迟到支付据此转异常收款而不是恢复订单。

func isClosedStatus(status string) bool {
	return status == model.OrderStatusClosed || status == model.OrderStatusCancelled
}

// statusLabel 订单状态中文名，用于通知留痕与审计描述。

func statusLabel(status string) string {
	switch status {
	case model.OrderStatusPendingPayment:
		return "待付款"
	case model.OrderStatusCancelled:
		return "取消"
	case model.OrderStatusClosed:
		return "关闭"
	case model.OrderStatusPaid:
		return "付款"
	case model.OrderStatusShipped:
		return "发货"
	case model.OrderStatusRefundRequested:
		return "退款申请"
	case model.OrderStatusRefundFailed:
		return "退款失败"
	case model.OrderStatusRefunded:
		return "退款"
	default:
		return status
	}
}

// itoa64 int64 版本的单号拼接辅助。

func itoa64(v int64) string { return itoa(int(v)) }
