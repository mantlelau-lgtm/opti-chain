package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Procurement order status constants.
const (
	POStatusDraft      = "DRAFT"
	POStatusApproved   = "APPROVED"
	POStatusInProgress = "IN_PROGRESS"
	POStatusCompleted  = "COMPLETED"
	POStatusCancelled  = "CANCELLED"
)

// PurchaseOrder corresponds to pur_order.
type PurchaseOrder struct {
	TenantBaseModel
	PONumber         string          `gorm:"column:po_number;size:64;index;not null" json:"po_number"`
	SupplierID       uint            `gorm:"column:supplier_id;not null;index" json:"supplier_id"`
	OrderDate        time.Time       `gorm:"column:order_date;not null" json:"order_date"`
	ExpectedDelivery *time.Time      `gorm:"column:expected_delivery_date" json:"expected_delivery_date"`
	TotalAmount      decimal.Decimal `gorm:"column:total_amount;type:decimal(14,2);default:0" json:"total_amount"`
	Status           string          `gorm:"column:status;size:32;default:DRAFT" json:"status"`

	Details []PurchaseOrderDetail `gorm:"foreignKey:POID" json:"details,omitempty"`
}

func (PurchaseOrder) TableName() string { return "pur_order" }

// PurchaseOrderDetail corresponds to pur_order_detail.
//
// received_qty accumulates ACCEPTED quantities only: rejected goods never
// count as received, so they remain "on order" for MRP until the supplier
// re-delivers (or the PO is closed manually).
type PurchaseOrderDetail struct {
	BaseModel
	POID        uint            `gorm:"column:po_id;not null;index" json:"po_id"`
	MaterialID  uint            `gorm:"column:material_id;not null;index" json:"material_id"`
	OrderQty    decimal.Decimal `gorm:"column:order_qty;type:decimal(12,4);not null" json:"order_qty"`
	UnitPrice   decimal.Decimal `gorm:"column:unit_price;type:decimal(12,4);not null" json:"unit_price"`
	ReceivedQty decimal.Decimal `gorm:"column:received_qty;type:decimal(12,4);default:0" json:"received_qty"`
	TotalPrice  decimal.Decimal `gorm:"column:total_price;type:decimal(14,2);not null" json:"total_price"`
}

func (PurchaseOrderDetail) TableName() string { return "pur_order_detail" }

// PurchaseReceipt corresponds to pur_receipt: one receiving round against a
// PO. A PO can be received over multiple receipts (partial delivery).
type PurchaseReceipt struct {
	TenantBaseModel
	ReceiptNumber string    `gorm:"column:receipt_number;size:64;index;not null" json:"receipt_number"`
	POID          uint      `gorm:"column:po_id;not null;index" json:"po_id"`
	WarehouseID   uint      `gorm:"column:warehouse_id;not null;index" json:"warehouse_id"`
	ReceiptDate   time.Time `gorm:"column:receipt_date;not null" json:"receipt_date"`
	Remark        string    `gorm:"column:remark;size:255" json:"remark"`

	Details []PurchaseReceiptDetail `gorm:"foreignKey:ReceiptID" json:"details,omitempty"`
}

func (PurchaseReceipt) TableName() string { return "pur_receipt" }

// PurchaseReceiptDetail corresponds to pur_receipt_detail. Passed qty enters
// stock; rejected qty stays out of stock and is tracked (with a reason) as the
// evidence for supplier return / re-delivery.
type PurchaseReceiptDetail struct {
	BaseModel
	ReceiptID    uint            `gorm:"column:receipt_id;not null;index" json:"receipt_id"`
	PODetailID   uint            `gorm:"column:po_detail_id;not null;index" json:"po_detail_id"`
	MaterialID   uint            `gorm:"column:material_id;not null;index" json:"material_id"`
	LocationID   uint            `gorm:"column:location_id;default:0" json:"location_id"`
	ReceivedQty  decimal.Decimal `gorm:"column:received_qty;type:decimal(12,4);not null" json:"received_qty"`
	PassedQty    decimal.Decimal `gorm:"column:passed_qty;type:decimal(12,4);not null;default:0" json:"passed_qty"`
	RejectedQty  decimal.Decimal `gorm:"column:rejected_qty;type:decimal(12,4);not null;default:0" json:"rejected_qty"`
	RejectReason string          `gorm:"column:reject_reason;size:255" json:"reject_reason"`
}

func (PurchaseReceiptDetail) TableName() string { return "pur_receipt_detail" }
