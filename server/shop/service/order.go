package service

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"shop/shop/model"
	"shop/shop/utils"
)

// OrderService 下单、取消、超时关闭与查询

type OrderService struct {
	deps Deps
}

// Create 下单：条件扣减可售数量 → 订单快照 → 库存流水，全部在一个事务内。

func (s OrderService) Create(buyerID, productID uint64, qty int) (model.Order, error) {
	var order model.Order
	if qty < 1 || qty > 2 {
		return order, ErrQtyInvalid
	}
	now := s.deps.Now()
	err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
		var p model.Product
		if err := tx.Where("id = ?", productID).First(&p).Error; err != nil {
			return ErrProductNotFound
		}
		// 关键并发闸门：仅当在售且可售数量足够时才扣减，靠 InnoDB 行锁保证互斥。
		res := tx.Model(&model.Product{}).
			Where("id = ? AND on_sale = ? AND available >= ?", productID, true, qty).
			Update("available", gorm.Expr("available - ?", qty))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			if !p.OnSale {
				return ErrProductOffSale
			}
			return ErrInsufficientStock
		}
		order = model.Order{
			OrderNo:     s.uniqueOrderNo(tx, now),
			BuyerID:     buyerID,
			ProductID:   productID,
			ProductName: p.Name,
			PriceCents:  p.PriceCents,
			Qty:         qty,
			AmountCents: p.PriceCents * int64(qty),
			Status:      model.OrderStatusPendingPayment,
			ExpireAt:    now.Add(s.deps.Cfg.OrderTimeout()),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		if err := appendLedger(tx, productID, order.OrderNo, model.LedgerOccupy, -qty, now); err != nil {
			return err
		}
		return logOp(tx, &order.OrderNo, &buyerID, model.RoleBuyer, "order_create",
			"下单占用:"+order.ProductName+" x"+itoa(qty))
	})
	if err != nil {
		return model.Order{}, err
	}
	return order, nil
}

// Cancel 买家取消待付款订单：状态条件更新成功才释放占用，重复关闭不会多释放。

func (s OrderService) Cancel(buyerID uint64, orderNo string) (model.Order, error) {
	now := s.deps.Now()
	orderNo = strings.TrimSpace(orderNo)
	var out model.Order
	err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
		var o model.Order
		if err := tx.Where("order_no = ?", orderNo).First(&o).Error; err != nil {
			return ErrOrderNotFound
		}
		if o.BuyerID != buyerID {
			return ErrForbidden
		}
		res := tx.Model(&model.Order{}).
			Where("order_no = ? AND buyer_id = ? AND status = ?", orderNo, buyerID, model.OrderStatusPendingPayment).
			Updates(map[string]interface{}{"status": model.OrderStatusCancelled, "closed_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return stateError(tx, orderNo)
		}
		if err := releaseStock(tx, o, model.LedgerRelease, now); err != nil {
			return err
		}
		if err := logOp(tx, &orderNo, &buyerID, model.RoleBuyer, "order_cancel", "买家主动取消并释放占用"); err != nil {
			return err
		}
		return tx.Where("order_no = ?", orderNo).First(&out).Error
	})
	if err != nil {
		return model.Order{}, err
	}
	return out, nil
}

// CloseExpired 超时关闭：以服务端时间为准，逐单条件更新后释放占用。

func (s OrderService) CloseExpired() (int, error) {
	now := s.deps.Now()
	var candidates []model.Order
	if err := s.deps.DB.Where("status = ? AND expire_at <= ?", model.OrderStatusPendingPayment, now).
		Order("id asc").Limit(200).Find(&candidates).Error; err != nil {
		return 0, err
	}
	closed := 0
	for _, c := range candidates {
		err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&model.Order{}).
				Where("order_no = ? AND status = ? AND expire_at <= ?", c.OrderNo, model.OrderStatusPendingPayment, now).
				Updates(map[string]interface{}{"status": model.OrderStatusClosed, "closed_at": now, "updated_at": now})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return nil // 已被支付或已取消，跳过，不释放数量
			}
			if err := releaseStock(tx, c, model.LedgerRelease, now); err != nil {
				return err
			}
			return logOp(tx, &c.OrderNo, nil, "system", "order_timeout_close", "服务端超时关闭并释放占用")
		})
		if err != nil {
			return closed, err
		}
		closed++
	}
	return closed, nil
}

// Get 按单号读取订单（不做视图权限判断，调用方负责）

func (s OrderService) Get(orderNo string) (model.Order, error) {
	var o model.Order
	if err := s.deps.DB.Where("order_no = ?", strings.TrimSpace(orderNo)).First(&o).Error; err != nil {
		return o, ErrOrderNotFound
	}
	return o, nil
}

// ListByBuyer 买家自己的订单

func (s OrderService) ListByBuyer(buyerID uint64, status string) ([]model.Order, error) {
	var list []model.Order
	q := s.deps.DB.Where("buyer_id = ?", buyerID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("id desc").Find(&list).Error
	return list, err
}

// ListByStatus 员工视角按状态查询订单

func (s OrderService) ListByStatus(status string, limit int) ([]model.Order, error) {
	var list []model.Order
	q := s.deps.DB
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if limit <= 0 {
		limit = 100
	}
	err := q.Order("id desc").Limit(limit).Find(&list).Error
	return list, err
}

// releaseStock 释放/归还可售数量：以该单库存流水净额为守恒依据，杜绝重复释放造成的虚增。
// 说明：运营允许把可售数量覆盖到高于 initial_stock（如活动补货），因此不能用 initial_stock 作为上界，
// 而是校验「该订单仍有数量处于占用状态」（净额为负）。

func releaseStock(tx *gorm.DB, o model.Order, changeType string, now time.Time) error {
	var net int
	if err := tx.Model(&model.InventoryLedger{}).
		Select("COALESCE(SUM(delta), 0)").Where("order_no = ?", o.OrderNo).Scan(&net).Error; err != nil {
		return err
	}
	if net > -o.Qty {
		// 该单占用已归还过，重复释放会虚增可售数量，直接回滚
		return NewBizError(2009, "库存归还不合法：可能已重复释放")
	}
	if err := tx.Model(&model.Product{}).
		Where("id = ?", o.ProductID).
		Update("available", gorm.Expr("available + ?", o.Qty)).Error; err != nil {
		return err
	}
	return appendLedger(tx, o.ProductID, o.OrderNo, changeType, o.Qty, now)
}

// stateError 根据订单当前状态给出准确原因，避免笼统报错。

func stateError(tx *gorm.DB, orderNo string) error {
	var o model.Order
	if err := tx.Where("order_no = ?", orderNo).First(&o).Error; err != nil {
		return ErrOrderNotFound
	}
	switch o.Status {
	case model.OrderStatusPaid, model.OrderStatusShipped, model.OrderStatusRefundRequested,
		model.OrderStatusRefundFailed, model.OrderStatusRefunded:
		return ErrAlreadyPaid
	case model.OrderStatusClosed, model.OrderStatusCancelled:
		return ErrOrderClosed
	default:
		return ErrOrderNotPayable
	}
}

// uniqueOrderNo 生成未占用的订单号（极端碰撞下重试）

func (s OrderService) uniqueOrderNo(tx *gorm.DB, now time.Time) string {
	for i := 0; i < 5; i++ {
		no := utils.NewOrderNo(now)
		var count int64
		tx.Model(&model.Order{}).Where("order_no = ?", no).Count(&count)
		if count == 0 {
			return no
		}
	}
	return utils.NewOrderNo(now.Add(time.Duration(1) * time.Nanosecond))
}

// itoa 整数转字符串（避免引入 fmt，错误码拼接路径高频调用）。

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
