package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Audit status constants (supplier qualification / customer access control).
const (
	AuditPending  = "PENDING"
	AuditApproved = "APPROVED"
	AuditRejected = "REJECTED"
)

// Customer corresponds to base_customer. credit_limit <= 0 means credit
// control is disabled for the customer.
type Customer struct {
	TenantBaseModel
	CustomerCode  string          `gorm:"column:customer_code;size:64;index;not null" json:"customer_code"`
	Name          string          `gorm:"column:name;size:128;not null" json:"name"`
	ContactPerson string          `gorm:"column:contact_person;size:64" json:"contact_person"`
	Phone         string          `gorm:"column:phone;size:32" json:"phone"`
	CreditLimit   decimal.Decimal `gorm:"column:credit_limit;type:decimal(14,2);default:0" json:"credit_limit"`
	UsedCredit    decimal.Decimal `gorm:"column:used_credit;type:decimal(14,2);default:0" json:"used_credit"`
	AuditStatus   string          `gorm:"column:audit_status;size:32;default:APPROVED" json:"audit_status"`
	Status        int8            `gorm:"column:status;default:1" json:"status"`
}

func (Customer) TableName() string { return "base_customer" }

// Sales order status constants.
const (
	SOStatusDraft      = "DRAFT"
	SOStatusApproved   = "APPROVED"
	SOStatusInShipping = "IN_SHIPPING"
	SOStatusCompleted  = "COMPLETED"
	SOStatusCancelled  = "CANCELLED"
)

// SaleOrder corresponds to sale_order. Approval locks stock (anti-oversell)
// and consumes customer credit; cancellation reverses both.
type SaleOrder struct {
	TenantBaseModel
	SONumber    string          `gorm:"column:so_number;size:64;index;not null" json:"so_number"`
	CustomerID  uint            `gorm:"column:customer_id;not null;index" json:"customer_id"`
	OrderDate   time.Time       `gorm:"column:order_date;not null" json:"order_date"`
	TotalAmount decimal.Decimal `gorm:"column:total_amount;type:decimal(14,2);default:0" json:"total_amount"`
	Status      string          `gorm:"column:status;size:32;default:DRAFT" json:"status"`
	CreatedBy   string          `gorm:"column:created_by;size:64" json:"created_by"`

	Details []SaleOrderDetail `gorm:"foreignKey:SOID" json:"details,omitempty"`
}

func (SaleOrder) TableName() string { return "sale_order" }

// SaleOrderDetail corresponds to sale_order_detail.
type SaleOrderDetail struct {
	BaseModel
	SOID       uint            `gorm:"column:so_id;not null;index" json:"so_id"`
	MaterialID uint            `gorm:"column:material_id;not null;index" json:"material_id"`
	Qty        decimal.Decimal `gorm:"column:qty;type:decimal(12,4);not null" json:"qty"`
	UnitPrice  decimal.Decimal `gorm:"column:unit_price;type:decimal(12,4);not null" json:"unit_price"`
	ShippedQty decimal.Decimal `gorm:"column:shipped_qty;type:decimal(12,4);default:0" json:"shipped_qty"`
	TotalPrice decimal.Decimal `gorm:"column:total_price;type:decimal(14,2);not null" json:"total_price"`
}

func (SaleOrderDetail) TableName() string { return "sale_order_detail" }
