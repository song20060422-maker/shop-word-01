package service

import (
	"strings"

	"gorm.io/gorm"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/model"
)

// ShipmentService 仓库发货：只为合法待发货订单登记一次。
type ShipmentService struct {
	deps Deps
}

// Shipable 待发货列表：仅 status=paid（退款申请/已退款/未付款都不在其中）
func (s ShipmentService) Shipable() ([]model.Order, error) {
	var list []model.Order
	err := s.deps.DB.Where("status = ?", model.OrderStatusPaid).Order("id asc").Find(&list).Error
	return list, err
}

// Ship 登记发货：状态条件更新 + 唯一键双保险，确保同一订单只发一次。
func (s ShipmentService) Ship(whID uint64, orderNo, carrier, trackingNo string) (model.Order, error) {
	orderNo = strings.TrimSpace(orderNo)
	carrier = strings.TrimSpace(carrier)
	trackingNo = strings.TrimSpace(trackingNo)
	if orderNo == "" || carrier == "" || trackingNo == "" {
		return model.Order{}, ErrInvalidParam
	}
	now := s.deps.Now()
	var out model.Order
	err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
		var o model.Order
		if err := tx.Where("order_no = ?", orderNo).First(&o).Error; err != nil {
			return ErrOrderNotFound
		}
		res := tx.Model(&model.Order{}).
			Where("order_no = ? AND status = ?", orderNo, model.OrderStatusPaid).
			Updates(map[string]interface{}{"status": model.OrderStatusShipped, "shipped_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return shipStateError(o.Status)
		}
		shipment := model.Shipment{
			OrderNo:    orderNo,
			Carrier:    carrier,
			TrackingNo: trackingNo,
			CreatedBy:  &whID,
			CreatedAt:  now,
		}
		if err := tx.Create(&shipment).Error; err != nil {
			return ErrAlreadyShipped
		}
		if err := appendLedger(tx, o.ProductID, orderNo, model.LedgerShipOut, 0, now); err != nil {
			return err
		}
		if err := logOp(tx, &orderNo, &whID, model.RoleWarehouse, "shipment_create",
			"发货登记："+carrier+" "+trackingNo); err != nil {
			return err
		}
		return tx.Where("order_no = ?", orderNo).First(&out).Error
	})
	if err != nil {
		return model.Order{}, err
	}
	return out, nil
}

// Get 查询发货记录（买家可查运单）
func (s ShipmentService) Get(orderNo string) (*model.Shipment, error) {
	var sh model.Shipment
	if err := s.deps.DB.Where("order_no = ?", strings.TrimSpace(orderNo)).First(&sh).Error; err != nil {
		return nil, nil
	}
	return &sh, nil
}

// List 全部发货记录（运营核查）
func (s ShipmentService) List() ([]model.Shipment, error) {
	var list []model.Shipment
	err := s.deps.DB.Order("id desc").Limit(200).Find(&list).Error
	return list, err
}

// shipStateError 把订单当前状态翻译为仓库看得懂的原因。
func shipStateError(status string) error {
	switch status {
	case model.OrderStatusRefundRequested:
		return ErrRefundInProgress
	case model.OrderStatusRefunded:
		return ErrRefunded
	case model.OrderStatusRefundFailed:
		return NewBizError(4011, "退款渠道处理失败，订单暂不可发货")
	case model.OrderStatusPendingPayment:
		return ErrNotPaid
	case model.OrderStatusClosed, model.OrderStatusCancelled:
		return ErrOrderClosed
	case model.OrderStatusShipped:
		return ErrAlreadyShipped
	default:
		return ErrNotShippable
	}
}
