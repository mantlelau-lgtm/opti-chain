// Package database owns the GORM engine: connection, dialect switching and
// schema migration. It is the single place that talks to the underlying DB.
package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"scm/internal/model"
)

// DB wraps a *gorm.DB with lifecycle helpers.
type DB struct {
	*gorm.DB
}

// Open builds a GORM engine for the configured dialect and auto-migrates schema.
func Open(driver, dsn string) (*DB, error) {
	var dialector gorm.Dialector
	switch driver {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "sqlite", "":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	gdb, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Cap the connection pool (important for the MySQL driver).
	if sqlDB, err := gdb.DB(); err == nil {
		sqlDB.SetMaxOpenConns(10)
	}

	// AutoMigrate compiles the models to the correct DDL for the active
	// dialect (SQLite/MySQL), keeping the two in sync automatically.
	if err := Migrate(gdb); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &DB{gdb}, nil
}

// Migrate creates/updates all tables defined in the model package.
func Migrate(gdb *gorm.DB) error {
	return gdb.AutoMigrate(
		&model.Material{},
		&model.Supplier{},
		&model.Warehouse{},
		&model.Location{},
		&model.PurchaseOrder{},
		&model.PurchaseOrderDetail{},
		&model.PurchaseReceipt{},
		&model.PurchaseReceiptDetail{},
		&model.Stock{},
		&model.InventoryOrder{},
		&model.InventoryOrderDetail{},
		&model.TransactionLog{},
		&model.Demand{},
		&model.MrpResult{},
		&model.Customer{},
		&model.SaleOrder{},
		&model.SaleOrderDetail{},
	)
}
