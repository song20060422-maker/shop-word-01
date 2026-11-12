package service

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/model"
	"github.com/flipped-aurora/gin-vue-admin/server/shop/utils"
)

// RefundService 售前退款与迟到支付退款
type RefundService struct {
	deps Deps
}

// Apply 买家对「已付款未发货」订单申请整单退款；申请期间订单不在待发货列表，天然暂停发货。
func (s RefundService) Apply(buyerID uint64, orderNo, reason string) (model.Refund, error) {
	now := s.deps.Now()
	orderNo = strings.TrimSpace(orderNo)
	var out model.Refund
	err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
		var o model.Order
		if err := tx.Where("order_no = ?", orderNo).First(&o).Error; err != nil {
			return ErrOrderNotFound
		}
		if o.BuyerID != buyerID {
			return ErrForbidden
		}
		res := tx.Model(&model.Order{}).
			Where("order_no = ? AND buyer_id = ? AND status = ?", orderNo, buyerID, model.OrderStatusPaid).
			Updates(map[string]interface{}{"status": model.OrderStatusRefundRequested, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			switch o.Status {
			case model.OrderStatusShipped:
				return NewBizError(4010, "订单已发货，不受理售前退款")
			case model.OrderStatusRefunded:
				return ErrRefunded
			case model.OrderStatusRefundRequested, model.OrderStatusRefundFailed:
				return ErrRefundExists
			case model.OrderStatusPendingPayment:
				return ErrNotPaid
			case model.OrderStatusClosed, model.OrderStatusCancelled:
				return ErrOrderClosed
			default:
				return ErrOrderNotPayable
			}
		}

		var existing model.Refund
		findErr := tx.Where("order_no = ?", orderNo).First(&existing).Error
		if findErr == nil {
			// 历史上被客服拒绝的单据可重新申请，复用同一张退款单（整单退款一单一张）
			if existing.ChannelStatus == model.ChannelSuccess {
				return ErrRefundExists
			}
			if err := tx.Model(&model.Refund{}).Where("id = ?", existing.ID).
				Updates(map[string]interface{}{
					"audit_status": model.AuditPending,
					"audit_reason": "",
					"audit_by":     nil,
					"audit_at":     nil,
					"updated_at":   now,
				}).Error; err != nil {
				return err
			}
			out = existing
			out.AuditStatus = model.AuditPending
			return logOp(tx, &orderNo, &buyerID, model.RoleBuyer, "refund_apply", "重新申请售前退款")
		}

		out = model.Refund{
			RefundNo:      s.uniqueRefundNo(tx, now),
			OrderNo:       orderNo,
			Source:        model.RefundSourcePresale,
			AmountCents:   o.AmountCents,
			AuditStatus:   model.AuditPending,
			AuditReason:   reason,
			ChannelStatus: model.ChannelPending,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Create(&out).Error; err != nil {
			return ErrRefundExists
		}
		return logOp(tx, &orderNo, &buyerID, model.RoleBuyer, "refund_apply", "买家申请售前退款："+reason)
	})
	if err != nil {
		return model.Refund{}, err
	}
	return out, nil
}

// Audit 客服审核：同意则调用渠道退款，拒绝则订单回到可发货状态。
func (s RefundService) Audit(csID uint64, orderNo string, approve bool, reason string) (model.Refund, error) {
	now := s.deps.Now()
	orderNo = strings.TrimSpace(orderNo)
	var refund model.Refund
	err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("order_no = ?", orderNo).First(&refund).Error; err != nil {
			return ErrRefundNotFound
		}
		if refund.AuditStatus != model.AuditPending {
			return ErrRefundAudited
		}
		status := model.AuditRejected
		if approve {
			status = model.AuditApproved
		}
		res := tx.Model(&model.Refund{}).
			Where("order_no = ? AND audit_status = ?", orderNo, model.AuditPending).
			Updates(map[string]interface{}{
				"audit_status": status,
				"audit_reason": reason,
				"audit_by":     csID,
				"audit_at":     now,
				"updated_at":   now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrRefundAudited
		}
		if !approve {
			// 拒绝后允许仓库继续发货
			if err := tx.Model(&model.Order{}).
				Where("order_no = ? AND status = ?", orderNo, model.OrderStatusRefundRequested).
				Updates(map[string]interface{}{"status": model.OrderStatusPaid, "updated_at": now}).Error; err != nil {
				return err
			}
			return logOp(tx, &orderNo, &csID, model.RoleCS, "refund_reject", "客服拒绝退款："+reason)
		}
		return logOp(tx, &orderNo, &csID, model.RoleCS, "refund_approve", "客服同意退款，转渠道处理")
	})
	if err != nil {
		return model.Refund{}, err
	}
	if !approve {
		refund.AuditStatus = model.AuditRejected
		return refund, nil
	}
	return s.callChannel(refund, csID, now)
}

// HandleAbnormal 客服处理迟到支付：发起异常退款，且不恢复已释放的数量。
func (s RefundService) HandleAbnormal(csID uint64, txnID, reason string) (model.Refund, error) {
	now := s.deps.Now()
	txnID = strings.TrimSpace(txnID)
	var refund model.Refund
	var abnormalID uint64
	err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
		var ab model.AbnormalPayment
		if err := tx.Where("channel_txn_id = ?", txnID).First(&ab).Error; err != nil {
			return ErrAbnormalNotFound
		}
		if ab.Status == model.AbnormalHandled {
			return NewBizError(3006, "该异常收款已处理")
		}
		var o model.Order
		if err := tx.Where("order_no = ?", ab.OrderNo).First(&o).Error; err != nil {
			return ErrOrderNotFound
		}
		var existing model.Refund
		findErr := tx.Where("order_no = ?", ab.OrderNo).First(&existing).Error
		if findErr == nil {
			// 整单退款一单一张：上次渠道失败留下的退款单直接复用重试，不得重复建单
			if existing.ChannelStatus == model.ChannelSuccess {
				return ErrRefundExists
			}
			if err := tx.Model(&model.Refund{}).Where("id = ?", existing.ID).
				Updates(map[string]interface{}{
					"audit_status":   model.AuditApproved,
					"audit_reason":   reason,
					"audit_by":       csID,
					"audit_at":       now,
					"channel_status": model.ChannelPending,
					"updated_at":     now,
				}).Error; err != nil {
				return err
			}
			refund = existing
			refund.AuditStatus = model.AuditApproved
			refund.AuditReason = reason
			refund.ChannelStatus = model.ChannelPending
		} else {
			refund = model.Refund{
				RefundNo:      s.uniqueRefundNo(tx, now),
				OrderNo:       ab.OrderNo,
				Source:        model.RefundSourceAbnormalLate,
				AmountCents:   ab.AmountCents,
				AuditStatus:   model.AuditApproved, // 客服已核实才发起，无需二次审核
				AuditReason:   reason,
				AuditBy:       &csID,
				AuditAt:       &now,
				ChannelStatus: model.ChannelPending,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := tx.Create(&refund).Error; err != nil {
				return ErrRefundExists
			}
		}
		abnormalID = ab.ID
		return logOp(tx, &ab.OrderNo, &csID, model.RoleCS, "abnormal_refund", "针对迟到支付发起退款："+reason)
	})
	if err != nil {
		return model.Refund{}, err
	}

	ok, _ := s.deps.Channel.Refund(refund.RefundNo, refund.AmountCents)
	err = s.deps.DB.Transaction(func(tx *gorm.DB) error {
		channelStatus := model.ChannelFailed
		if ok {
			channelStatus = model.ChannelSuccess
		}
		if err := tx.Model(&model.Refund{}).Where("refund_no = ?", refund.RefundNo).
			Updates(map[string]interface{}{"channel_status": channelStatus, "updated_at": now}).Error; err != nil {
			return err
		}
		if ok {
			// 迟到支付场景：订单早已关闭、数量早已释放，这里绝不再恢复库存
			if err := tx.Model(&model.AbnormalPayment{}).Where("id = ?", abnormalID).
				Updates(map[string]interface{}{
					"status":     model.AbnormalHandled,
					"handled_by": csID,
					"handled_at": now,
					"refund_id":  refund.ID,
				}).Error; err != nil {
				return err
			}
			return logOp(tx, &refund.OrderNo, &csID, model.RoleCS, "abnormal_refund_success",
				"异常收款退款成功，未重复释放库存")
		}
		return logOp(tx, &refund.OrderNo, &csID, model.RoleCS, "abnormal_refund_failed",
			"渠道退款失败，异常记录保留待处理")
	})
	if err != nil {
		return model.Refund{}, err
	}
	refund.ChannelStatus = model.ChannelSuccess
	if !ok {
		refund.ChannelStatus = model.ChannelFailed
		return refund, ErrChannelRefund
	}
	return refund, nil
}

// Retry 渠道明确失败后由客服重试
func (s RefundService) Retry(csID uint64, refundNo string) (model.Refund, error) {
	now := s.deps.Now()
	var refund model.Refund
	if err := s.deps.DB.Where("refund_no = ?", strings.TrimSpace(refundNo)).First(&refund).Error; err != nil {
		return refund, ErrRefundNotFound
	}
	if refund.AuditStatus != model.AuditApproved || refund.ChannelStatus != model.ChannelFailed {
		return refund, ErrRefundNotRetryable
	}
	return s.callChannel(refund, csID, now)
}

// callChannel 调用模拟渠道并根据结果推进订单、退款单与库存。
func (s RefundService) callChannel(refund model.Refund, csID uint64, now time.Time) (model.Refund, error) {
	ok, err := s.deps.Channel.Refund(refund.RefundNo, refund.AmountCents)
	if err != nil {
		ok = false
	}
	var result model.Refund
	err = s.deps.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("refund_no = ?", refund.RefundNo).First(&result).Error; err != nil {
			return ErrRefundNotFound
		}
		retry := result.RetryCount + 1
		if !ok {
			if err := tx.Model(&model.Refund{}).Where("refund_no = ?", refund.RefundNo).
				Updates(map[string]interface{}{
					"channel_status": model.ChannelFailed,
					"retry_count":    retry,
					"updated_at":     now,
				}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.Order{}).
				Where("order_no = ? AND status = ?", refund.OrderNo, model.OrderStatusRefundRequested).
				Updates(map[string]interface{}{"status": model.OrderStatusRefundFailed, "updated_at": now}).Error; err != nil {
				return err
			}
			return logOp(tx, &refund.OrderNo, &csID, model.RoleCS, "refund_channel_failed",
				"渠道退款失败，保留待处理记录等待重试")
		}

		// 渠道成功：订单进入已退款；库存恢复受 restored 闸门保护，只做一次
		if err := tx.Model(&model.Order{}).
			Where("order_no = ? AND status IN ?", refund.OrderNo,
				[]string{model.OrderStatusRefundRequested, model.OrderStatusRefundFailed}).
			Updates(map[string]interface{}{"status": model.OrderStatusRefunded, "updated_at": now}).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"channel_status": model.ChannelSuccess,
			"retry_count":    retry,
			"updated_at":     now,
		}
		restore := false
		if result.Source == model.RefundSourcePresale && !result.Restored {
			gate := tx.Model(&model.Refund{}).
				Where("refund_no = ? AND restored = ?", refund.RefundNo, false).
				Update("restored", true)
			if gate.Error != nil {
				return gate.Error
			}
			restore = gate.RowsAffected == 1
			if restore {
				updates["restored"] = true
			}
		}
		if err := tx.Model(&model.Refund{}).Where("refund_no = ?", refund.RefundNo).Updates(updates).Error; err != nil {
			return err
		}
		if restore {
			var o model.Order
			if err := tx.Where("order_no = ?", refund.OrderNo).First(&o).Error; err != nil {
				return err
			}
			if err := releaseStock(tx, o, model.LedgerRestore, now); err != nil {
				return err
			}
		}
		return logOp(tx, &refund.OrderNo, &csID, model.RoleCS, "refund_success",
			"渠道退款成功；库存恢复="+boolLabel(restore))
	})
	if err != nil {
		return model.Refund{}, err
	}
	if err := s.deps.DB.Where("refund_no = ?", refund.RefundNo).First(&result).Error; err != nil {
		return model.Refund{}, err
	}
	if !ok {
		return result, ErrChannelRefund
	}
	return result, nil
}

// Get 按单号查询退款单
func (s RefundService) Get(orderNo string) (model.Refund, error) {
	var r model.Refund
	if err := s.deps.DB.Where("order_no = ?", strings.TrimSpace(orderNo)).First(&r).Error; err != nil {
		return r, ErrRefundNotFound
	}
	return r, nil
}

// List 客服工作台的退款单列表
func (s RefundService) List(status string) ([]model.Refund, error) {
	var list []model.Refund
	q := s.deps.DB
	if status != "" {
		q = q.Where("audit_status = ? OR channel_status = ?", status, status)
	}
	err := q.Order("id desc").Limit(100).Find(&list).Error
	return list, err
}

func (s RefundService) uniqueRefundNo(tx *gorm.DB, now time.Time) string {
	for i := 0; i < 5; i++ {
		no := utils.NewRefundNo(now)
		var count int64
		tx.Model(&model.Refund{}).Where("refund_no = ?", no).Count(&count)
		if count == 0 {
			return no
		}
	}
	return utils.NewRefundNo(now.Add(time.Nanosecond))
}

func boolLabel(v bool) string {
	if v {
		return "是"
	}
	return "否"
}
