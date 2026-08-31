package database

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// legacyUniqueColumns maps table -> column whose single-column UNIQUE index
// predates multi-tenancy. After AutoMigrate adds tenant_id, uniqueness must
// become (tenant_id, column); GORM never drops the old index, so we do.
var legacyUniqueColumns = map[string]string{
	"sys_material":  "sku_code",
	"sys_supplier":  "supplier_code",
	"base_customer": "customer_code",
	"sys_warehouse": "warehouse_code",
	"sys_location":  "location_code",
	"pur_order":     "po_number",
	"pur_receipt":   "receipt_number",
	"sale_order":    "so_number",
	"inv_order":     "order_number",
	"plan_demand":   "demand_number",
	"sys_user":      "username",
}

// MigrateTenantIndexes upgrades single-column unique indexes to composite
// (tenant_id, column) ones. SQLite supports IF EXISTS / IF NOT EXISTS; MySQL
// is assumed fresh (no legacy indexes) so failures there are ignored.
func MigrateTenantIndexes(db *gorm.DB, driver string) {
	for table, col := range legacyUniqueColumns {
		legacy := fmt.Sprintf("idx_%s_%s", table, col)
		composite := fmt.Sprintf("uk_%s_tenant_%s", table, col)
		if driver == "sqlite" {
			db.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %q", legacy))
			if err := db.Exec(fmt.Sprintf(
				"CREATE UNIQUE INDEX IF NOT EXISTS %q ON %q(tenant_id, %s)",
				composite, table, col)).Error; err != nil {
				log.Printf("migrate index %s: %v", composite, err)
			}
			continue
		}
		// MySQL: best-effort, fresh deployments have no legacy index.
		db.Exec(fmt.Sprintf("ALTER TABLE %s DROP INDEX %s", table, legacy))
		db.Exec(fmt.Sprintf(
			"CREATE UNIQUE INDEX %s ON %s(tenant_id, %s)", composite, table, col))
	}
}
