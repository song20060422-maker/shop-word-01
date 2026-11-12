// Package model 定义 GORM 数据模型与业务枚举。
// 表结构由 migrations/*.sql 建立（保留 InnoDB 的 ENUM / CHECK / UNIQUE 约束），
// 模型仅做列映射，因此每张表都显式声明 TableName，避免 GORM 复数化推断偏差。
package model

import "time"

// 订单状态机：pending_payment -> paid -> shipped / refund_requested -> refunded|refund_failed
const (
	OrderStatusPendingPayment  = "pending_payment"
	OrderStatusCancelled       = "cancelled"
	OrderStatusClosed          = "closed"
	OrderStatusPaid            = "paid"
	OrderStatusShipped         = "shipped"
	OrderStatusRefundRequested = "refund_requested"
	OrderStatusRefundFailed    = "refund_failed"
	OrderStatusRefunded        = "refunded"
)

// 角色
const (
	RoleBuyer     = "buyer"
	RoleOps       = "ops"
	RoleWarehouse = "warehouse"
	RoleCS        = "cs"
)

// 退款来源：售前退款 / 迟到支付退款
const (
	RefundSourcePresale      = "presale"
	RefundSourceAbnormalLate = "abnormal_late"
)

// 退款审核与渠道状态
const (
	AuditPending  = "pending"
	AuditApproved = "approved"
	AuditRejected = "rejected"

	ChannelPending = "pending"
	ChannelSuccess = "success"
	ChannelFailed  = "failed"
)

// 支付通知处理结果
const (
	NoticeAccepted   = "accepted"
	NoticeRejected   = "rejected"
	NoticeDuplicated = "duplicated"
	NoticeAbnormal   = "abnormal"
)

// 库存流水类型
const (
	LedgerOccupy  = "occupy"
	LedgerRelease = "release"
	LedgerRestore = "restore"
	LedgerShipOut = "ship_out"
)

// 异常支付处理状态
const (
	AbnormalPending = "pending"
	AbnormalHandled = "handled"
)

// User 买家与员工账号
type User struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Username     string    `gorm:"column:username" json:"username"`
	PasswordHash string    `gorm:"column:password_hash" json:"-"`
	Role         string    `gorm:"column:role" json:"role"`
	DisplayName  string    `gorm:"column:display_name" json:"displayName"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (User) TableName() string { return "users" }

// Product 商品与可售数量
type Product struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SKU          string    `gorm:"column:sku" json:"sku"`
	Name         string    `gorm:"column:name" json:"name"`
	PriceCents   int64     `gorm:"column:price_cents" json:"priceCents"`
	Available    int       `gorm:"column:available" json:"available"`
	InitialStock int       `gorm:"column:initial_stock" json:"initialStock"`
	OnSale       bool      `gorm:"column:on_sale" json:"onSale"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (Product) TableName() string { return "products" }

// Order 订单（含商品与价格快照）
type Order struct {
	ID          uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderNo     string     `gorm:"column:order_no" json:"orderNo"`
	BuyerID     uint64     `gorm:"column:buyer_id" json:"buyerId"`
	ProductID   uint64     `gorm:"column:product_id" json:"productId"`
	ProductName string     `gorm:"column:product_name" json:"productName"`
	PriceCents  int64      `gorm:"column:price_cents" json:"priceCents"`
	Qty         int        `gorm:"column:qty" json:"qty"`
	AmountCents int64      `gorm:"column:amount_cents" json:"amountCents"`
	Status      string     `gorm:"column:status" json:"status"`
	ExpireAt    time.Time  `gorm:"column:expire_at" json:"expireAt"`
	PaidAt      *time.Time `gorm:"column:paid_at" json:"paidAt,omitempty"`
	ShippedAt   *time.Time `gorm:"column:shipped_at" json:"shippedAt,omitempty"`
	ClosedAt    *time.Time `gorm:"column:closed_at" json:"closedAt,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (Order) TableName() string { return "orders" }

// Payment 成功记款（幂等基石：order_no 与 channel_txn_id 双唯一）
type Payment struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderNo      string    `gorm:"column:order_no" json:"orderNo"`
	ChannelTxnID string    `gorm:"column:channel_txn_id" json:"channelTxnId"`
	AmountCents  int64     `gorm:"column:amount_cents" json:"amountCents"`
	PaidAt       time.Time `gorm:"column:paid_at" json:"paidAt"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (Payment) TableName() string { return "payments" }

// PaymentNotice 支付通知原始留痕（含被拒与重复）
type PaymentNotice struct {
	ID           uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderNo      string    `gorm:"column:order_no" json:"orderNo"`
	ChannelTxnID string    `gorm:"column:channel_txn_id" json:"channelTxnId"`
	AmountCents  int64     `gorm:"column:amount_cents" json:"amountCents"`
	SignOK       bool      `gorm:"column:sign_ok" json:"signOk"`
	Result       string    `gorm:"column:result" json:"result"`
	Reason       string    `gorm:"column:reason" json:"reason"`
	RawBody      *string   `gorm:"column:raw_body" json:"-"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (PaymentNotice) TableName() string { return "payment_notices" }

// Refund 退款单（整单退款，一单一张）
type Refund struct {
	ID            uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	RefundNo      string     `gorm:"column:refund_no" json:"refundNo"`
	OrderNo       string     `gorm:"column:order_no" json:"orderNo"`
	Source        string     `gorm:"column:source" json:"source"`
	AmountCents   int64      `gorm:"column:amount_cents" json:"amountCents"`
	AuditStatus   string     `gorm:"column:audit_status" json:"auditStatus"`
	AuditReason   string     `gorm:"column:audit_reason" json:"auditReason"`
	AuditBy       *uint64    `gorm:"column:audit_by" json:"auditBy,omitempty"`
	AuditAt       *time.Time `gorm:"column:audit_at" json:"auditAt,omitempty"`
	ChannelStatus string     `gorm:"column:channel_status" json:"channelStatus"`
	RetryCount    int        `gorm:"column:retry_count" json:"retryCount"`
	Restored      bool       `gorm:"column:restored" json:"restored"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (Refund) TableName() string { return "refunds" }

// Shipment 发货登记
type Shipment struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderNo    string    `gorm:"column:order_no" json:"orderNo"`
	Carrier    string    `gorm:"column:carrier" json:"carrier"`
	TrackingNo string    `gorm:"column:tracking_no" json:"trackingNo"`
	CreatedBy  *uint64   `gorm:"column:created_by" json:"createdBy,omitempty"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (Shipment) TableName() string { return "shipments" }

// AbnormalPayment 迟到支付等异常收款待处理
type AbnormalPayment struct {
	ID           uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderNo      string     `gorm:"column:order_no" json:"orderNo"`
	ChannelTxnID string     `gorm:"column:channel_txn_id" json:"channelTxnId"`
	AmountCents  int64      `gorm:"column:amount_cents" json:"amountCents"`
	Reason       string     `gorm:"column:reason" json:"reason"`
	Status       string     `gorm:"column:status" json:"status"`
	HandledBy    *uint64    `gorm:"column:handled_by" json:"handledBy,omitempty"`
	HandledAt    *time.Time `gorm:"column:handled_at" json:"handledAt,omitempty"`
	RefundID     *uint64    `gorm:"column:refund_id" json:"refundId,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"createdAt"`
}

func (AbnormalPayment) TableName() string { return "abnormal_payments" }

// InventoryLedger 库存变动流水（守恒校验依据）
type InventoryLedger struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ProductID      uint64    `gorm:"column:product_id" json:"productId"`
	OrderNo        string    `gorm:"column:order_no" json:"orderNo"`
	ChangeType     string    `gorm:"column:change_type" json:"changeType"`
	Delta          int       `gorm:"column:delta" json:"delta"`
	AvailableAfter int       `gorm:"column:available_after" json:"availableAfter"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (InventoryLedger) TableName() string { return "inventory_ledger" }

// OperationLog 操作留痕（追加写，不提供修改与删除接口）
type OperationLog struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	OrderNo   *string   `gorm:"column:order_no" json:"orderNo,omitempty"`
	ActorID   *uint64   `gorm:"column:actor_id" json:"actorId,omitempty"`
	ActorRole string    `gorm:"column:actor_role" json:"actorRole"`
	Action    string    `gorm:"column:action" json:"action"`
	Detail    string    `gorm:"column:detail" json:"detail"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
}

func (OperationLog) TableName() string { return "operation_logs" }
