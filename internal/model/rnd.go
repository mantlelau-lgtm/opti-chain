package model

import "github.com/shopspring/decimal"

// BOM status constants.
const (
	BOMStatusDraft    = "DRAFT"
	BOMStatusReleased = "RELEASED"
	BOMStatusObsolete = "OBSOLETE"
)

// Product corresponds to base_product: an R&D product spec (finished good).
// Products are NOT stocked in inv_stock; their BOM drives material
// procurement via MRP.
type Product struct {
	TenantBaseModel
	ProductCode string          `gorm:"column:product_code;size:64;index;not null" json:"product_code"`
	Name        string          `gorm:"column:name;size:128;not null" json:"name"`
	Spec        string          `gorm:"column:spec;size:128" json:"spec"`
	Unit        string          `gorm:"column:unit;size:32;not null" json:"unit"`
	CostPrice   decimal.Decimal `gorm:"column:cost_price;type:decimal(14,2);default:0" json:"cost_price"`
	Status      int8            `gorm:"column:status;default:1" json:"status"`
}

func (Product) TableName() string { return "base_product" }

// BOM corresponds to base_bom: a versioned bill of materials for a product.
// One RELEASED version is the effective (is_default) one per product.
type BOM struct {
	TenantBaseModel
	BOMNo     string          `gorm:"column:bom_no;size:64;index;not null" json:"bom_no"`
	ProductID uint            `gorm:"column:product_id;not null;index" json:"product_id"`
	Version   int             `gorm:"column:version;not null;default:1" json:"version"`
	Status    string          `gorm:"column:status;size:32;default:DRAFT" json:"status"`
	IsDefault bool            `gorm:"column:is_default;default:false" json:"is_default"`
	UnitQty   decimal.Decimal `gorm:"column:unit_qty;type:decimal(12,4);default:1" json:"unit_qty"`
	Remark    string          `gorm:"column:remark;size:255" json:"remark"`

	Details []BOMDetail `gorm:"foreignKey:BOMID" json:"details,omitempty"`
}

func (BOM) TableName() string { return "base_bom" }

// BOMDetail corresponds to base_bom_detail: one component line.
type BOMDetail struct {
	BaseModel
	BOMID       uint            `gorm:"column:bom_id;not null;index" json:"bom_id"`
	ComponentID uint            `gorm:"column:component_id;not null;index" json:"component_id"`
	QtyPerUnit  decimal.Decimal `gorm:"column:qty_per_unit;type:decimal(12,4);not null" json:"qty_per_unit"`
	ScrapRate   decimal.Decimal `gorm:"column:scrap_rate;type:decimal(12,4);default:0" json:"scrap_rate"`
	Remark      string          `gorm:"column:remark;size:255" json:"remark"`
}

func (BOMDetail) TableName() string { return "base_bom_detail" }
