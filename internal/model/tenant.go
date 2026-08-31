package model

import "time"

// Tenant status constants.
const (
	TenantActive    = "ACTIVE"
	TenantSuspended = "SUSPENDED"
)

// Tenant corresponds to sys_tenant. Platform-level table (not tenant-scoped).
type Tenant struct {
	BaseModel
	Code      string     `gorm:"column:code;size:64;uniqueIndex;not null" json:"code"`
	Name      string     `gorm:"column:name;size:128;not null" json:"name"`
	Plan      string     `gorm:"column:plan;size:32;default:FREE" json:"plan"`
	Status    string     `gorm:"column:status;size:32;default:ACTIVE" json:"status"`
	ExpiresAt *time.Time `gorm:"column:expires_at" json:"expires_at"`
}

func (Tenant) TableName() string { return "sys_tenant" }
