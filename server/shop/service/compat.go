package service

import (
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"shop/shop/model"
)

// 本文件为「接口契约兼容层」补齐的 service 扩展方法。
// 只新增方法，不修改任何既有方法签名与行为，保证原有 /api 原型接口语义不变。

// ProductBySKU 按 SKU 查询商品：契约层以 SKU 作为对外商品 id。

func (s CatalogService) ProductBySKU(sku string) (model.Product, error) {
	var p model.Product
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return p, ErrProductNotFound
	}
	if err := s.deps.DB.Where("sku = ?", sku).First(&p).Error; err != nil {
		return p, ErrProductNotFound
	}
	return p, nil
}

// OverwriteAvailable 运营直接覆盖可售数量（契约约定：available 为绝对值，而非增量）。
// 与占用/释放路径一致地追加库存流水，便于运营核对数量来源。

func (s CatalogService) OverwriteAvailable(actorID uint64, actorRole string, productID uint64, available int) (model.Product, error) {
	if available < 0 {
		return model.Product{}, NewBizError(1006, "可售数量不能小于 0")
	}
	now := s.deps.Now()
	err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
		var p model.Product
		if err := tx.Where("id = ?", productID).First(&p).Error; err != nil {
			return ErrProductNotFound
		}
		if p.Available == available {
			return logOp(tx, nil, &actorID, actorRole, "product_stock_overwrite",
				"运营覆盖可售数量："+strconv.Itoa(available)+"（无变化）")
		}
		if err := tx.Model(&model.Product{}).Where("id = ?", productID).
			Updates(map[string]interface{}{"available": available, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := appendLedger(tx, productID, "", "restore", available-p.Available, now); err != nil {
			return err
		}
		return logOp(tx, nil, &actorID, actorRole, "product_stock_overwrite",
			"运营覆盖可售数量："+strconv.Itoa(p.Available)+" -> "+strconv.Itoa(available))
	})
	if err != nil {
		return model.Product{}, err
	}
	return s.Get(productID)
}

// FindByRef 订单双寻址：先按 order_no，再按数字主键（契约 {id} 同时兼容两者）。

func (s OrderService) FindByRef(ref string) (model.Order, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return model.Order{}, ErrOrderNotFound
	}
	var o model.Order
	if err := s.deps.DB.Where("order_no = ?", ref).First(&o).Error; err == nil {
		return o, nil
	}
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil && id > 0 {
		if err := s.deps.DB.Where("id = ?", id).First(&o).Error; err == nil {
			return o, nil
		}
	}
	return model.Order{}, ErrOrderNotFound
}

// PendingOrderNos 待付款订单号列表，按创建时间倒序（最新在前）。

func (s OrderService) PendingOrderNos() ([]string, error) {
	nos := make([]string, 0, 8)
	err := s.deps.DB.Model(&model.Order{}).
		Where("status = ?", model.OrderStatusPendingPayment).
		Order("id desc").Pluck("order_no", &nos).Error
	return nos, err
}

// ForceExpire 把指定订单的到期时间提前到 at 之前（测试钩子：让最新一笔待付款订单立即到期）。

func (s OrderService) ForceExpire(orderNo string, at time.Time) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" {
		return ErrOrderNotFound
	}
	return s.deps.DB.Model(&model.Order{}).
		Where("order_no = ? AND status = ?", orderNo, model.OrderStatusPendingPayment).
		Updates(map[string]interface{}{
			"expire_at":  at.Add(-2 * time.Second),
			"updated_at": s.deps.Now(),
		}).Error
}

// FindByRef 退款单三寻址：refund_no / order_no / 数字主键（契约 {id} 传退款单号）。

func (s RefundService) FindByRef(ref string) (model.Refund, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return model.Refund{}, ErrRefundNotFound
	}
	var r model.Refund
	if err := s.deps.DB.Where("refund_no = ?", ref).First(&r).Error; err == nil {
		return r, nil
	}
	if err := s.deps.DB.Where("order_no = ?", ref).Order("id desc").First(&r).Error; err == nil {
		return r, nil
	}
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil && id > 0 {
		if err := s.deps.DB.Where("id = ?", id).First(&r).Error; err == nil {
			return r, nil
		}
	}
	return model.Refund{}, ErrRefundNotFound
}

// PaymentsByOrder 查询订单的成功收款记录（契约 /api/ops/payments 的记款部分）。

func (s PaymentService) PaymentsByOrder(orderNo string) ([]model.Payment, error) {
	list := make([]model.Payment, 0, 2)
	err := s.deps.DB.Where("order_no = ?", strings.TrimSpace(orderNo)).
		Order("id asc").Find(&list).Error
	return list, err
}

// AbnormalByID 按主键查询异常收款记录（契约 /api/cs/payment-exceptions/{id}/refund 使用）。

func (s PaymentService) AbnormalByID(id uint64) (model.AbnormalPayment, error) {
	var ab model.AbnormalPayment
	if err := s.deps.DB.Where("id = ?", id).First(&ab).Error; err != nil {
		return ab, ErrAbnormalNotFound
	}
	return ab, nil
}
