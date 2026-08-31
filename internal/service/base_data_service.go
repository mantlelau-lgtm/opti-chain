package service

import (
	"scm/internal/model"
	"scm/internal/repository"
)

// Every method takes the tenant id first (threaded from the authenticated
// Actor by the handler layer) so data never crosses tenant boundaries.

// ---- Material ----

type MaterialService struct{ repo *repository.MaterialRepo }

func NewMaterialService(repo *repository.MaterialRepo) *MaterialService {
	return &MaterialService{repo: repo}
}

func (s *MaterialService) Create(t uint, m *model.Material) error {
	if m.SKUCode == "" || m.Name == "" || m.Unit == "" {
		return errorsBadRequest("sku_code/name/unit are required")
	}
	return s.repo.Create(t, m)
}

func (s *MaterialService) Update(t, id uint, m *model.Material) error {
	m.ID = id
	return s.repo.Update(t, m)
}

func (s *MaterialService) Get(t, id uint) (*model.Material, error) {
	return s.repo.Get(t, id)
}

func (s *MaterialService) Delete(t, id uint) error {
	return s.repo.Delete(t, id)
}

func (s *MaterialService) List(t uint, in PageInput) ([]model.Material, int64, error) {
	var (
		out   []model.Material
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ---- Supplier ----

type SupplierService struct{ repo *repository.SupplierRepo }

func NewSupplierService(repo *repository.SupplierRepo) *SupplierService {
	return &SupplierService{repo: repo}
}

func (s *SupplierService) Create(t uint, m *model.Supplier) error {
	if m.SupplierCode == "" || m.Name == "" {
		return errorsBadRequest("supplier_code/name are required")
	}
	return s.repo.Create(t, m)
}

func (s *SupplierService) Update(t, id uint, m *model.Supplier) error {
	m.ID = id
	return s.repo.Update(t, m)
}

func (s *SupplierService) Get(t, id uint) (*model.Supplier, error) {
	return s.repo.Get(t, id)
}

func (s *SupplierService) Delete(t, id uint) error {
	return s.repo.Delete(t, id)
}

func (s *SupplierService) List(t uint, in PageInput) ([]model.Supplier, int64, error) {
	var (
		out   []model.Supplier
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// SetAuditStatus transitions a supplier's qualification state (RACI 准入).
// Only APPROVED suppliers may be used on purchase orders.
func (s *SupplierService) SetAuditStatus(t, id uint, status string) (*model.Supplier, error) {
	if status != model.AuditPending && status != model.AuditApproved && status != model.AuditRejected {
		return nil, errorsBadRequest("audit_status must be PENDING, APPROVED or REJECTED")
	}
	m, err := s.repo.Get(t, id)
	if m == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	m.AuditStatus = status
	if err := s.repo.Update(t, m); err != nil {
		return nil, err
	}
	return m, nil
}

// ---- Warehouse ----

type WarehouseService struct{ repo *repository.WarehouseRepo }

func NewWarehouseService(repo *repository.WarehouseRepo) *WarehouseService {
	return &WarehouseService{repo: repo}
}

func (s *WarehouseService) Create(t uint, m *model.Warehouse) error {
	if m.WarehouseCode == "" || m.Name == "" {
		return errorsBadRequest("warehouse_code/name are required")
	}
	return s.repo.Create(t, m)
}

func (s *WarehouseService) Update(t, id uint, m *model.Warehouse) error {
	m.ID = id
	return s.repo.Update(t, m)
}

func (s *WarehouseService) Get(t, id uint) (*model.Warehouse, error) {
	return s.repo.Get(t, id)
}

func (s *WarehouseService) Delete(t, id uint) error {
	return s.repo.Delete(t, id)
}

func (s *WarehouseService) List(t uint, in PageInput) ([]model.Warehouse, int64, error) {
	var (
		out   []model.Warehouse
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ---- Location ----

type LocationService struct{ repo *repository.LocationRepo }

func NewLocationService(repo *repository.LocationRepo) *LocationService {
	return &LocationService{repo: repo}
}

func (s *LocationService) Create(t uint, m *model.Location) error {
	if m.LocationCode == "" || m.Name == "" {
		return errorsBadRequest("location_code/name are required")
	}
	return s.repo.Create(t, m)
}

func (s *LocationService) Update(t, id uint, m *model.Location) error {
	m.ID = id
	return s.repo.Update(t, m)
}

func (s *LocationService) Get(t, id uint) (*model.Location, error) {
	return s.repo.Get(t, id)
}

func (s *LocationService) Delete(t, id uint) error {
	return s.repo.Delete(t, id)
}

func (s *LocationService) List(t uint, in PageInput) ([]model.Location, int64, error) {
	var (
		out   []model.Location
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
