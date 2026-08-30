package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Demand source constants.
const (
	DemandSourceForecast   = "FORECAST"
	DemandSourceSalesOrder = "SALES_ORDER"
)

// Demand status constants.
const (
	DemandStatusOpen      = "OPEN"
	DemandStatusGenerated = "GENERATED"
)

// MRP result status constants.
const (
	MrpStatusPending   = "PENDING"
	MrpStatusConverted = "CONVERTED"
)

// Demand corresponds to plan_demand.
type Demand struct {
	BaseModel
	DemandNumber string          `gorm:"column:demand_number;size:64;uniqueIndex;not null" json:"demand_number"`
	MaterialID   uint            `gorm:"column:material_id;not null;index" json:"material_id"`
	DemandQty    decimal.Decimal `gorm:"column:demand_qty;type:decimal(12,4);not null" json:"demand_qty"`
	DemandDate   time.Time       `gorm:"column:demand_date;not null" json:"demand_date"`
	SourceType   string          `gorm:"column:source_type;size:32;not null" json:"source_type"`
	Status       string          `gorm:"column:status;size:32;default:OPEN" json:"status"`
}

func (Demand) TableName() string { return "plan_demand" }

// MrpResult corresponds to plan_mrp_result.
type MrpResult struct {
	BaseModel
	MrpNumber       string          `gorm:"column:mrp_number;size:64;index" json:"mrp_number"`
	MaterialID      uint            `gorm:"column:material_id;not null;index" json:"material_id"`
	CurrentStock    decimal.Decimal `gorm:"column:current_stock;type:decimal(12,4);default:0" json:"current_stock"`
	OnOrderQty      decimal.Decimal `gorm:"column:on_order_qty;type:decimal(12,4);default:0" json:"on_order_qty"`
	GrossDemand     decimal.Decimal `gorm:"column:gross_demand;type:decimal(12,4);not null" json:"gross_demand"`
	SuggestedPOQty  decimal.Decimal `gorm:"column:suggested_po_qty;type:decimal(12,4);not null" json:"suggested_po_qty"`
	SuggestedPODate *time.Time      `gorm:"column:suggested_po_date" json:"suggested_po_date"`
	Status          string          `gorm:"column:status;size:32;default:PENDING" json:"status"`
}

func (MrpResult) TableName() string { return "plan_mrp_result" }
