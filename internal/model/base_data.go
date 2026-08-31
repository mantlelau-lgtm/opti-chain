package model

import "github.com/shopspring/decimal"

// Material corresponds to sys_material.
type Material struct {
	TenantBaseModel
	SKUCode  string          `gorm:"column:sku_code;size:64;index;not null" json:"sku_code"`
	Name     string          `gorm:"column:name;size:128;not null" json:"name"`
	Category string          `gorm:"column:category;size:64;not null" json:"category"`
	Unit     string          `gorm:"column:unit;size:16;not null" json:"unit"`
	MinStock decimal.Decimal `gorm:"column:min_stock;type:decimal(12,4);default:0" json:"min_stock"`
	MaxStock decimal.Decimal `gorm:"column:max_stock;type:decimal(12,4);default:0" json:"max_stock"`
	Status   int8            `gorm:"column:status;default:1" json:"status"`
}

// TableName overrides the default pluralized name.
func (Material) TableName() string { return "sys_material" }

// Supplier corresponds to sys_supplier. audit_status gates purchasing: only
// APPROVED suppliers may appear on a purchase order (RACI 准入管控).
type Supplier struct {
	TenantBaseModel
	SupplierCode  string `gorm:"column:supplier_code;size:64;index;not null" json:"supplier_code"`
	Name          string `gorm:"column:name;size:128;not null" json:"name"`
	ContactPerson string `gorm:"column:contact_person;size:64" json:"contact_person"`
	Phone         string `gorm:"column:phone;size:32" json:"phone"`
	Address       string `gorm:"column:address;size:255" json:"address"`
	AuditStatus   string `gorm:"column:audit_status;size:32;default:PENDING" json:"audit_status"`
	Status        int8   `gorm:"column:status;default:1" json:"status"`
}

func (Supplier) TableName() string { return "sys_supplier" }

// Warehouse corresponds to sys_warehouse.
type Warehouse struct {
	TenantBaseModel
	WarehouseCode string `gorm:"column:warehouse_code;size:32;index;not null" json:"warehouse_code"`
	Name          string `gorm:"column:name;size:64;not null" json:"name"`
	Address       string `gorm:"column:address;size:255" json:"address"`
	Status        int8   `gorm:"column:status;default:1" json:"status"`
}

func (Warehouse) TableName() string { return "sys_warehouse" }

// Location corresponds to sys_location.
type Location struct {
	TenantBaseModel
	WarehouseID  uint   `gorm:"column:warehouse_id;not null;index" json:"warehouse_id"`
	LocationCode string `gorm:"column:location_code;size:32;not null;index" json:"location_code"`
	Name         string `gorm:"column:name;size:64" json:"name"`
	Status       int8   `gorm:"column:status;default:1" json:"status"`
}

func (Location) TableName() string { return "sys_location" }
