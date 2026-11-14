package service

import (
	"strings"

	"shop/shop/model"
	"shop/shop/model/response"
)

// ReportService 订单聚合视图与运营核查汇总

type ReportService struct {
	deps Deps
}

// OrderDetail 聚合订单、收款、退款、发货、异常与操作轨迹，买家可据此知道钱和货去了哪里。

func (s ReportService) OrderDetail(orderNo string, viewerID uint64, viewerRole string) (response.OrderDetail, error) {
	var detail response.OrderDetail
	orderNo = strings.TrimSpace(orderNo)
	if err := s.deps.DB.Where("order_no = ?", orderNo).First(&detail.Order).Error; err != nil {
		return detail, ErrOrderNotFound
	}
	if viewerRole == model.RoleBuyer && detail.Order.BuyerID != viewerID {
		return response.OrderDetail{}, ErrForbidden
	}

	var pay model.Payment
	if err := s.deps.DB.Where("order_no = ?", orderNo).First(&pay).Error; err == nil {
		detail.Payment = &pay
	}
	var refund model.Refund
	if err := s.deps.DB.Where("order_no = ?", orderNo).First(&refund).Error; err == nil {
		detail.Refund = &refund
	}
	var shipment model.Shipment
	if err := s.deps.DB.Where("order_no = ?", orderNo).First(&shipment).Error; err == nil {
		detail.Shipment = &shipment
	}
	var product model.Product
	if err := s.deps.DB.Where("id = ?", detail.Order.ProductID).First(&product).Error; err == nil {
		detail.ProductInfo = &product
	}
	_ = s.deps.DB.Where("order_no = ?", orderNo).Order("id asc").Find(&detail.Abnormal).Error
	_ = s.deps.DB.Where("order_no = ?", orderNo).Order("id asc").Find(&detail.Notices).Error
	_ = s.deps.DB.Where("order_no = ?", orderNo).Order("id asc").Find(&detail.Timeline).Error

	detail.ShipableNow = detail.Order.Status == model.OrderStatusPaid
	detail.RefundPending = detail.Order.Status == model.OrderStatusRefundRequested ||
		detail.Order.Status == model.OrderStatusRefundFailed
	return detail, nil
}

// Summary 运营核查：商品可售数量、已收款、已退款与订单状态分布。

func (s ReportService) Summary() (response.TradeSummary, error) {
	out := response.TradeSummary{StatusCount: map[string]int64{}}

	var products []model.Product
	if err := s.deps.DB.Order("id asc").Find(&products).Error; err != nil {
		return out, err
	}

	type aggRow struct {
		ProductID uint64
		Qty       int64
		Cents     int64
	}
	var shipRows []aggRow
	if err := s.deps.DB.Model(&model.Order{}).
		Select("product_id AS product_id, COALESCE(SUM(qty),0) AS qty").
		Where("status = ?", model.OrderStatusShipped).
		Group("product_id").Scan(&shipRows).Error; err != nil {
		return out, err
	}
	var paidRows []aggRow
	if err := s.deps.DB.Table("payments").
		Select("orders.product_id AS product_id, COALESCE(SUM(payments.amount_cents),0) AS cents").
		Joins("JOIN orders ON orders.order_no = payments.order_no").
		Group("orders.product_id").Scan(&paidRows).Error; err != nil {
		return out, err
	}
	var refundRows []aggRow
	if err := s.deps.DB.Table("refunds").
		Select("orders.product_id AS product_id, COALESCE(SUM(refunds.amount_cents),0) AS cents").
		Joins("JOIN orders ON orders.order_no = refunds.order_no").
		Where("refunds.channel_status = ?", model.ChannelSuccess).
		Group("orders.product_id").Scan(&refundRows).Error; err != nil {
		return out, err
	}
	// 迟到支付：渠道已收款但订单已关闭，不会写入 payments；金额按异常收款记录计入，
	// 避免"渠道已收款而商城报表查不到"的盲区（题面规则 8）。
	var abnormalRows []aggRow
	if err := s.deps.DB.Table("abnormal_payments").
		Select("orders.product_id AS product_id, COALESCE(SUM(abnormal_payments.amount_cents),0) AS cents").
		Joins("JOIN orders ON orders.order_no = abnormal_payments.order_no").
		Group("orders.product_id").Scan(&abnormalRows).Error; err != nil {
		return out, err
	}
	var abnormalTotal int64
	if err := s.deps.DB.Model(&model.AbnormalPayment{}).
		Select("COALESCE(SUM(amount_cents),0)").Scan(&abnormalTotal).Error; err != nil {
		return out, err
	}

	shippedOf := func(id uint64) int64 {
		for _, r := range shipRows {
			if r.ProductID == id {
				return r.Qty
			}
		}
		return 0
	}
	paidOf := func(id uint64) int64 {
		var cents int64
		for _, r := range paidRows {
			if r.ProductID == id {
				cents += r.Cents
			}
		}
		for _, r := range abnormalRows {
			if r.ProductID == id {
				cents += r.Cents
			}
		}
		return cents
	}
	refundedOf := func(id uint64) int64 {
		for _, r := range refundRows {
			if r.ProductID == id {
				return r.Cents
			}
		}
		return 0
	}

	for _, p := range products {
		shipped := shippedOf(p.ID)
		occupied := p.InitialStock - p.Available - int(shipped)
		if occupied < 0 {
			occupied = 0
		}
		out.Products = append(out.Products, response.ProductStat{
			ProductID:     p.ID,
			SKU:           p.SKU,
			Name:          p.Name,
			PriceCents:    p.PriceCents,
			Available:     p.Available,
			InitialStock:  p.InitialStock,
			OnSale:        p.OnSale,
			Occupied:      occupied,
			ShippedQty:    int(shipped),
			PaidCents:     paidOf(p.ID),
			RefundedCents: refundedOf(p.ID),
		})
	}

	if err := s.deps.DB.Model(&model.Payment{}).
		Select("COALESCE(SUM(amount_cents),0)").Scan(&out.TotalPaidCents).Error; err != nil {
		return out, err
	}
	out.TotalPaidCents += abnormalTotal
	if err := s.deps.DB.Model(&model.Refund{}).
		Select("COALESCE(SUM(amount_cents),0)").
		Where("channel_status = ?", model.ChannelSuccess).Scan(&out.TotalRefundedCents).Error; err != nil {
		return out, err
	}
	if err := s.deps.DB.Model(&model.Order{}).Count(&out.OrderCount).Error; err != nil {
		return out, err
	}

	type statusRow struct {
		Status string
		Total  int64
	}
	var rows []statusRow
	if err := s.deps.DB.Model(&model.Order{}).Select("status AS status, COUNT(*) AS total").
		Group("status").Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, r := range rows {
		out.StatusCount[r.Status] = r.Total
	}
	return out, nil
}

// Trail 订单全链路操作留痕（只读，异常不能被改状态抹掉）

func (s ReportService) Trail(orderNo string) ([]model.OperationLog, error) {
	var list []model.OperationLog
	q := s.deps.DB
	if strings.TrimSpace(orderNo) != "" {
		q = q.Where("order_no = ?", strings.TrimSpace(orderNo))
	}
	err := q.Order("id asc").Limit(500).Find(&list).Error
	return list, err
}

// Ledger 库存流水（核对占用/释放/恢复/出库是否守恒）

func (s ReportService) Ledger(productID uint64) ([]model.InventoryLedger, error) {
	var list []model.InventoryLedger
	q := s.deps.DB
	if productID > 0 {
		q = q.Where("product_id = ?", productID)
	}
	err := q.Order("id asc").Limit(500).Find(&list).Error
	return list, err
}
