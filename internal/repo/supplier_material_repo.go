package repository

import (
	"gorm.io/gorm"

	"scm/internal/model"
)

// SupplierMaterialRepo owns base_supplier_material.
type SupplierMaterialRepo struct{ *tenantRepo[model.SupplierMaterial] }

func NewSupplierMaterialRepo(db *gormDB) *SupplierMaterialRepo {
	return &SupplierMaterialRepo{tenantRepo: newTenantRepo[model.SupplierMaterial](db)}
}

// List returns relationships filtered by supplier and/or material (both
// optional; at least one should be given by the caller).
func (r *SupplierMaterialRepo) List(t, supplierID, materialID uint, out *[]model.SupplierMaterial) error {
	q := r.db.DB.Where("tenant_id = ?", t)
	if supplierID != 0 {
		q = q.Where("supplier_id = ?", supplierID)
	}
	if materialID != 0 {
		q = q.Where("material_id = ?", materialID)
	}
	return q.Order("id").Find(out).Error
}

// GetByPair finds the relationship for a (supplier, material) pair, nil if
// absent.
func (r *SupplierMaterialRepo) GetByPair(t, supplierID, materialID uint) (*model.SupplierMaterial, error) {
	var m model.SupplierMaterial
	err := r.db.DB.Where("tenant_id = ? AND supplier_id = ? AND material_id = ?", t, supplierID, materialID).
		First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}
