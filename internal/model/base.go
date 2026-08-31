// Package model defines the domain entities (GORM ORM structs).
//
// Type choices are intentionally cross-database friendly:
//   - INTEGER PRIMARY KEY AUTOINCREMENT  -> gorm `uint` + `autoIncrement`
//   - VARCHAR(n)                        -> `string` + `size:n`
//   - TINYINT                           -> `int8`
//   - DECIMAL(p, s)                     -> shopspring decimal.Decimal
//   - DATETIME                          -> time.Time
//
// GORM compiles these to the correct DDL per dialect (SQLite / MySQL) at
// AutoMigrate time, so no hand-written SQL differs between the two databases.
package model

import "time"

// BaseModel carries fields shared by most tables.
type BaseModel struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TenantBaseModel extends BaseModel with the tenant discriminator. All
// business/master-data tables embed it; composite uniqueness
// (tenant_id + code) is enforced by migration SQL, not GORM tags.
type TenantBaseModel struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  uint      `gorm:"column:tenant_id;not null;default:0;index" json:"tenant_id"`
	CreatedBy string    `gorm:"column:created_by;size:64" json:"created_by"`
	UpdatedBy string    `gorm:"column:updated_by;size:64" json:"updated_by"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// Tenanted is implemented (via embedding) by every tenant-scoped model so the
// repository layer can read/write the discriminator generically.
type Tenanted interface {
	SetTenantID(uint)
	GetTenantID() uint
}

func (m *TenantBaseModel) SetTenantID(t uint) { m.TenantID = t }
func (m *TenantBaseModel) GetTenantID() uint  { return m.TenantID }
