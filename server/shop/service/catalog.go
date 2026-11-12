package service

import (
	"time"

	"gorm.io/gorm"

	"github.com/flipped-aurora/gin-vue-admin/server/shop/model"
)

// CatalogService 商品上下架、可售数量与库存流水
type CatalogService struct {
	deps Deps
}

// OnSale 买家可见的在售商品
func (s CatalogService) OnSale() ([]model.Product, error) {
	var list []model.Product
	err := s.deps.DB.Where("on_sale = ?", true).Order("id asc").Find(&list).Error
	return list, err
}

// All 运营可见的全部商品
func (s CatalogService) All() ([]model.Product, error) {
	var list []model.Product
	err := s.deps.DB.Order("id asc").Find(&list).Error
	return list, err
}

// Get 单个商品
func (s CatalogService) Get(id uint64) (model.Product, error) {
	var p model.Product
	if err := s.deps.DB.Where("id = ?", id).First(&p).Error; err != nil {
		return p, ErrProductNotFound
	}
	return p, nil
}

// Update 运营维护上下架与售价；已存在订单的价格快照不受影响。
func (s CatalogService) Update(actorID uint64, actorRole string, id uint64, onSale *bool, price *int64) (model.Product, error) {
	if price != nil && *price <= 0 {
		return model.Product{}, NewBizError(1005, "售价必须大于 0")
	}
	err := s.deps.DB.Transaction(func(tx *gorm.DB) error {
		var p model.Product
		if err := tx.Where("id = ?", id).First(&p).Error; err != nil {
			return ErrProductNotFound
		}
		updates := map[string]interface{}{}
		if onSale != nil {
			updates["on_sale"] = *onSale
		}
		if price != nil {
			updates["price_cents"] = *price
		}
		if len(updates) > 0 {
			if err := tx.Model(&model.Product{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		detail := "运营维护商品"
		if onSale != nil {
			if *onSale {
				detail += "，上架"
			} else {
				detail += "，下架"
			}
		}
		if price != nil {
			detail += "，调整售价"
		}
		return logOp(tx, nil, &actorID, actorRole, "product_update", detail)
	})
	if err != nil {
		return model.Product{}, err
	}
	return s.Get(id)
}

// appendLedger 记录库存流水：先抢占/归还可售数量，再读回实时值写入 available_after。
func appendLedger(tx *gorm.DB, productID uint64, orderNo, changeType string, delta int, now time.Time) error {
	var available int
	if err := tx.Model(&model.Product{}).Select("available").Where("id = ?", productID).Scan(&available).Error; err != nil {
		return err
	}
	return tx.Create(&model.InventoryLedger{
		ProductID:      productID,
		OrderNo:        orderNo,
		ChangeType:     changeType,
		Delta:          delta,
		AvailableAfter: available,
		CreatedAt:      now,
	}).Error
}

// logOp 追加操作留痕（只增不改）
func logOp(tx *gorm.DB, orderNo *string, actorID *uint64, actorRole, action, detail string) error {
	return tx.Create(&model.OperationLog{
		OrderNo:   orderNo,
		ActorID:   actorID,
		ActorRole: actorRole,
		Action:    action,
		Detail:    detail,
	}).Error
}

// CurrentAvailable 读取商品当前可售数量（页面展示与演示断言使用）
func (s CatalogService) CurrentAvailable(id uint64) (int, error) {
	var available int
	if err := s.deps.DB.Model(&model.Product{}).Select("available").Where("id = ?", id).Scan(&available).Error; err != nil {
		return 0, err
	}
	return available, nil
}
