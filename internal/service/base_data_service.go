package service

import (
	"scm/internal/model"
	"scm/internal/repository"
)

// ---- Material ----

// MaterialService wraps MaterialRepo with validation.
type MaterialService struct{ repo *repository.MaterialRepo }

func NewMaterialService(repo *repository.MaterialRepo) *MaterialService {
	return &MaterialService{repo: repo}
}

// Create validates and persists a material.
func (s *MaterialService) Create(m *model.Material) error {
	if m.SKUCode == "" || m.Name == "" || m.Category == "" || m.Unit == "" {
		return errorsBadRequest("sku_code/name/category/unit are required")
	}
	return s.repo.Create(m)
}

// Update replaces an existing material.
func (s *MaterialService) Update(id uint, m *model.Material) error {
	m.ID = id
	return s.repo.Update(m)
}

// Get loads a material by id.
func (s *MaterialService) Get(id uint) (*model.Material, error) {
	return s.repo.Get(id)
}

// Delete removes a material.
func (s *MaterialService) Delete(id uint) error {
	return s.repo.Delete(id)
}

// List returns a paginated material list.
func (s *MaterialService) List(in PageInput) ([]model.Material, int64, error) {
	var (
		out   []model.Material
		total int64
	)
	if err := s.repo.List(repository.ListFilter{Page: in.Page, Keyword: in.Keyword}, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ---- Supplier ----

// SupplierService wraps SupplierRepo.
type SupplierService struct{ repo *repository.SupplierRepo }

func NewSupplierService(repo *repository.SupplierRepo) *SupplierService {
	return &SupplierService{repo: repo}
}

func (s *SupplierService) Create(m *model.Supplier) error {
	if m.SupplierCode == "" || m.Name == "" {
		return errorsBadRequest("supplier_code/name are required")
	}
	return s.repo.Create(m)
}

func (s *SupplierService) Update(id uint, m *model.Supplier) error {
	m.ID = id
	return s.repo.Update(m)
}

func (s *SupplierService) Get(id uint) (*model.Supplier, error) {
	return s.repo.Get(id)
}

func (s *SupplierService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *SupplierService) List(in PageInput) ([]model.Supplier, int64, error) {
	var (
		out   []model.Supplier
		total int64
	)
	if err := s.repo.List(repository.ListFilter{Page: in.Page, Keyword: in.Keyword}, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// SetAuditStatus transitions a supplier's qualification state (RACI 准入).
// Only APPROVED suppliers may be used on purchase orders.
func (s *SupplierService) SetAuditStatus(id uint, status string) (*model.Supplier, error) {
	if status != model.AuditPending && status != model.AuditApproved && status != model.AuditRejected {
		return nil, errorsBadRequest("audit_status must be PENDING, APPROVED or REJECTED")
	}
	m, err := s.repo.Get(id)
	if m == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	m.AuditStatus = status
	if err := s.repo.Update(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ---- Warehouse ----

// WarehouseService wraps WarehouseRepo.
type WarehouseService struct{ repo *repository.WarehouseRepo }

func NewWarehouseService(repo *repository.WarehouseRepo) *WarehouseService {
	return &WarehouseService{repo: repo}
}

func (s *WarehouseService) Create(m *model.Warehouse) error {
	if m.WarehouseCode == "" || m.Name == "" {
		return errorsBadRequest("warehouse_code/name are required")
	}
	return s.repo.Create(m)
}

func (s *WarehouseService) Update(id uint, m *model.Warehouse) error {
	m.ID = id
	return s.repo.Update(m)
}

func (s *WarehouseService) Get(id uint) (*model.Warehouse, error) {
	return s.repo.Get(id)
}

func (s *WarehouseService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *WarehouseService) List(in PageInput) ([]model.Warehouse, int64, error) {
	var (
		out   []model.Warehouse
		total int64
	)
	if err := s.repo.List(repository.ListFilter{Page: in.Page, Keyword: in.Keyword}, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ---- Location ----

// LocationService wraps LocationRepo.
type LocationService struct{ repo *repository.LocationRepo }

func NewLocationService(repo *repository.LocationRepo) *LocationService {
	return &LocationService{repo: repo}
}

func (s *LocationService) Create(m *model.Location) error {
	if m.WarehouseID == 0 || m.LocationCode == "" {
		return errorsBadRequest("warehouse_id/location_code are required")
	}
	return s.repo.Create(m)
}

func (s *LocationService) Update(id uint, m *model.Location) error {
	m.ID = id
	return s.repo.Update(m)
}

func (s *LocationService) Get(id uint) (*model.Location, error) {
	return s.repo.Get(id)
}

func (s *LocationService) Delete(id uint) error {
	return s.repo.Delete(id)
}

func (s *LocationService) List(in PageInput) ([]model.Location, int64, error) {
	var (
		out   []model.Location
		total int64
	)
	if err := s.repo.List(repository.ListFilter{Page: in.Page, Keyword: in.Keyword}, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
