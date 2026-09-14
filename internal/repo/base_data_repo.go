package repository

import (
	"scm/internal/model"
)

// ---- Material ----

type MaterialRepo struct{ *tenantRepo[model.Material] }

func NewMaterialRepo(db *gormDB) *MaterialRepo {
	return &MaterialRepo{tenantRepo: newTenantRepo[model.Material](db)}
}

func (r *MaterialRepo) List(f ListFilter, out *[]model.Material, total *int64) error {
	return r.listT(f, keywordLike(f, "sku_code", "name"), out, total)
}

// ---- Supplier ----

type SupplierRepo struct{ *tenantRepo[model.Supplier] }

func NewSupplierRepo(db *gormDB) *SupplierRepo {
	return &SupplierRepo{tenantRepo: newTenantRepo[model.Supplier](db)}
}

func (r *SupplierRepo) List(f ListFilter, out *[]model.Supplier, total *int64) error {
	return r.listT(f, keywordLike(f, "supplier_code", "name", "contact_person"), out, total)
}

// ---- Warehouse ----

type WarehouseRepo struct{ *tenantRepo[model.Warehouse] }

func NewWarehouseRepo(db *gormDB) *WarehouseRepo {
	return &WarehouseRepo{tenantRepo: newTenantRepo[model.Warehouse](db)}
}

func (r *WarehouseRepo) List(f ListFilter, out *[]model.Warehouse, total *int64) error {
	return r.listT(f, keywordLike(f, "warehouse_code", "name"), out, total)
}
