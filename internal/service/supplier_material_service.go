package service

import (
	"github.com/shopspring/decimal"

	"scm/internal/model"
	"scm/internal/repo"
)

// SupplierMaterialService owns the supplier↔material supply relationships and
// their prices.
type SupplierMaterialService struct {
	repo      *repository.SupplierMaterialRepo
	suppliers *repository.SupplierRepo
	materials *repository.MaterialRepo
}

func NewSupplierMaterialService(repo *repository.SupplierMaterialRepo, suppliers *repository.SupplierRepo, materials *repository.MaterialRepo) *SupplierMaterialService {
	return &SupplierMaterialService{repo: repo, suppliers: suppliers, materials: materials}
}

// BindInput is the payload for binding a material to a supplier with a price.
type BindInput struct {
	SupplierID   uint   `json:"supplier_id"`
	MaterialID   uint   `json:"material_id"`
	UnitPrice    string `json:"unit_price"`
	LeadTimeDays int    `json:"lead_time_days"`
	IsPreferred  bool   `json:"is_preferred"`
}

// List returns relationships for a supplier and/or a material.
func (s *SupplierMaterialService) List(t, supplierID, materialID uint) ([]model.SupplierMaterial, error) {
	var out []model.SupplierMaterial
	if err := s.repo.List(t, supplierID, materialID, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Bind creates or updates (upsert) the relationship for a supplier+material
// pair, so "binding" again just refreshes the price/lead time.
func (s *SupplierMaterialService) Bind(t uint, in BindInput) (*model.SupplierMaterial, error) {
	if in.SupplierID == 0 || in.MaterialID == 0 {
		return nil, errorsBadRequest("supplier_id and material_id are required")
	}
	if supplier, _ := s.suppliers.Get(t, in.SupplierID); supplier == nil {
		return nil, errNotFound(in.SupplierID)
	}
	if material, _ := s.materials.Get(t, in.MaterialID); material == nil {
		return nil, errNotFound(in.MaterialID)
	}
	price, err := decimal.NewFromString(in.UnitPrice)
	if err != nil || price.IsNegative() {
		return nil, errorsBadRequest("unit_price must be a non-negative number")
	}
	existing, err := s.repo.GetByPair(t, in.SupplierID, in.MaterialID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		existing.UnitPrice = price
		existing.LeadTimeDays = in.LeadTimeDays
		existing.IsPreferred = in.IsPreferred
		existing.Status = 1
		if err := s.repo.Update(t, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}
	m := &model.SupplierMaterial{
		SupplierID:   in.SupplierID,
		MaterialID:   in.MaterialID,
		UnitPrice:    price,
		LeadTimeDays: in.LeadTimeDays,
		IsPreferred:  in.IsPreferred,
		Status:       1,
	}
	if err := s.repo.Create(t, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Update edits an existing relationship line.
func (s *SupplierMaterialService) Update(t, id uint, in BindInput) (*model.SupplierMaterial, error) {
	m, err := s.repo.Get(t, id)
	if m == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	price, err := decimal.NewFromString(in.UnitPrice)
	if err != nil || price.IsNegative() {
		return nil, errorsBadRequest("unit_price must be a non-negative number")
	}
	m.UnitPrice = price
	m.LeadTimeDays = in.LeadTimeDays
	m.IsPreferred = in.IsPreferred
	if err := s.repo.Update(t, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Unbind removes a relationship.
func (s *SupplierMaterialService) Unbind(t, id uint) error {
	return s.repo.Delete(t, id)
}
