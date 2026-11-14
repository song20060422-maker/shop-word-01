package service

import (
	"gorm.io/gorm"

	"shop/shop/model"
)

// DemoService 演示数据重置：仅清理业务数据并恢复商品初始可售数量，保留账号。
// 由 shop.demo-mode 配置开关保护，避免生产误用。

type DemoService struct {
	deps Deps
}

// Reset 按外键依赖顺序清理业务表，并把商品可售数量恢复到初始库存。

func (s DemoService) Reset() error {
	return s.deps.DB.Transaction(func(tx *gorm.DB) error {
		// 先删子表，再删订单，避免外键约束失败
		tables := []string{
			"payments",
			"payment_notices",
			"refunds",
			"shipments",
			"abnormal_payments",
			"inventory_ledger",
			"operation_logs",
			"orders",
		}
		for _, t := range tables {
			if err := tx.Exec("DELETE FROM " + t).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Product{}).Where("1 = 1").
			Updates(map[string]interface{}{
				"available": gorm.Expr("initial_stock"),
				"on_sale":   true,
			}).Error; err != nil {
			return err
		}
		return nil
	})
}
