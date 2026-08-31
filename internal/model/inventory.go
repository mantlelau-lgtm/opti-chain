package model

import "github.com/shopspring/decimal"

// Inventory order type constants.
const (
	OrderTypePurchaseIn = "PURCHASE_IN"
	OrderTypeSaleOut    = "SALE_OUT"
	OrderTypeTransfer   = "TRANSFER"
)

// Inventory order status constants.
const (
	InvOrderPending   = "PENDING"
	InvOrderCompleted = "COMPLETED"
)

// Transaction action constants.
const (
	ActionIn  = "IN"
	ActionOut = "OUT"
)

// Stock corresponds to inv_stock (real-time on-hand inventory).
type Stock struct {
	TenantBaseModel
	WarehouseID    uint            `gorm:"column:warehouse_id;not null;index" json:"warehouse_id"`
	LocationID     uint            `gorm:"column:location_id;default:0" json:"location_id"`
	MaterialID     uint            `gorm:"column:material_id;not null;index" json:"material_id"`
	Quantity       decimal.Decimal `gorm:"column:quantity;type:decimal(12,4);default:0" json:"quantity"`
	LockedQuantity decimal.Decimal `gorm:"column:locked_quantity;type:decimal(12,4);default:0" json:"locked_quantity"`
}

func (Stock) TableName() string { return "inv_stock" }

// InventoryOrder corresponds to inv_order.
type InventoryOrder struct {
	TenantBaseModel
	OrderNumber    string `gorm:"column:order_number;size:64;index;not null" json:"order_number"`
	OrderType      string `gorm:"column:order_type;size:32;not null" json:"order_type"`
	RefOrderNumber string `gorm:"column:ref_order_number;size:64" json:"ref_order_number"`
	WarehouseID    uint   `gorm:"column:warehouse_id;not null;index" json:"warehouse_id"`
	Status         string `gorm:"column:status;size:32;default:PENDING" json:"status"`

	Details []InventoryOrderDetail `gorm:"foreignKey:InvOrderID" json:"details,omitempty"`
}

func (InventoryOrder) TableName() string { return "inv_order" }

// InventoryOrderDetail corresponds to inv_order_detail.
type InventoryOrderDetail struct {
	BaseModel
	InvOrderID uint            `gorm:"column:inv_order_id;not null;index" json:"inv_order_id"`
	MaterialID uint            `gorm:"column:material_id;not null;index" json:"material_id"`
	LocationID uint            `gorm:"column:location_id;default:0" json:"location_id"`
	Qty        decimal.Decimal `gorm:"column:qty;type:decimal(12,4);not null" json:"qty"`
}

func (InventoryOrderDetail) TableName() string { return "inv_order_detail" }

// TransactionLog corresponds to inv_transaction_log (audit trail).
type TransactionLog struct {
	TenantBaseModel
	MaterialID     uint            `gorm:"column:material_id;not null;index" json:"material_id"`
	WarehouseID    uint            `gorm:"column:warehouse_id;not null;index" json:"warehouse_id"`
	LocationID     uint            `gorm:"column:location_id;default:0" json:"location_id"`
	ActionType     string          `gorm:"column:action_type;size:32;not null" json:"action_type"`
	ChangeQty      decimal.Decimal `gorm:"column:change_qty;type:decimal(12,4);not null" json:"change_qty"`
	BeforeQty      decimal.Decimal `gorm:"column:before_qty;type:decimal(12,4);not null" json:"before_qty"`
	AfterQty       decimal.Decimal `gorm:"column:after_qty;type:decimal(12,4);not null" json:"after_qty"`
	RefOrderNumber string          `gorm:"column:ref_order_number;size:64" json:"ref_order_number"`
}

func (TransactionLog) TableName() string { return "inv_transaction_log" }
