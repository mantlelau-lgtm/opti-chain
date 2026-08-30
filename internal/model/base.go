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
