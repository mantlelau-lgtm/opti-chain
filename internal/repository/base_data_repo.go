package repository

import (
	"gorm.io/gorm"

	"scm/internal/model"
)

// ---- Material ----

type MaterialRepo struct{ *genericRepo[model.Material] }

func NewMaterialRepo(db *gormDB) *MaterialRepo {
	return &MaterialRepo{genericRepo: newGenericRepo[model.Material](db)}
}

func (r *MaterialRepo) List(f ListFilter, out *[]model.Material, total *int64) error {
	return r.list(f, keywordLike(f, "sku_code", "name"), out, total)
}

// ---- Supplier ----

type SupplierRepo struct{ *genericRepo[model.Supplier] }

func NewSupplierRepo(db *gormDB) *SupplierRepo {
	return &SupplierRepo{genericRepo: newGenericRepo[model.Supplier](db)}
}

func (r *SupplierRepo) List(f ListFilter, out *[]model.Supplier, total *int64) error {
	return r.list(f, keywordLike(f, "supplier_code", "name", "contact_person"), out, total)
}

// ---- Warehouse ----

type WarehouseRepo struct{ *genericRepo[model.Warehouse] }

func NewWarehouseRepo(db *gormDB) *WarehouseRepo {
	return &WarehouseRepo{genericRepo: newGenericRepo[model.Warehouse](db)}
}

func (r *WarehouseRepo) List(f ListFilter, out *[]model.Warehouse, total *int64) error {
	return r.list(f, keywordLike(f, "warehouse_code", "name"), out, total)
}

// ---- Location ----

type LocationRepo struct{ *genericRepo[model.Location] }

func NewLocationRepo(db *gormDB) *LocationRepo {
	return &LocationRepo{genericRepo: newGenericRepo[model.Location](db)}
}

func (r *LocationRepo) List(f ListFilter, out *[]model.Location, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("location_code LIKE ? OR name LIKE ?", like, like)
		}
		return q
	}
	return r.list(f, apply, out, total)
}
