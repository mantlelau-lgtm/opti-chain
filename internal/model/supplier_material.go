package model

import "github.com/shopspring/decimal"

// SupplierMaterial corresponds to base_supplier_material: a supply
// relationship binding a supplier to a material with the agreed price and
// lead time. Uniqueness (tenant, supplier, material) is enforced by the
// service via upsert semantics.
type SupplierMaterial struct {
	TenantBaseModel
	SupplierID   uint            `gorm:"column:supplier_id;not null;index" json:"supplier_id"`
	MaterialID   uint            `gorm:"column:material_id;not null;index" json:"material_id"`
	UnitPrice    decimal.Decimal `gorm:"column:unit_price;type:decimal(12,4);not null" json:"unit_price"`
	LeadTimeDays int             `gorm:"column:lead_time_days;default:0" json:"lead_time_days"`
	IsPreferred  bool            `gorm:"column:is_preferred;default:false" json:"is_preferred"`
	Status       int8            `gorm:"column:status;default:1" json:"status"`
}

func (SupplierMaterial) TableName() string { return "base_supplier_material" }
