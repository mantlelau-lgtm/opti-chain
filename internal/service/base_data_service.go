package service

import (
	"context"
	"fmt"
	"scm/internal/model"
	repository "scm/internal/repo"
)

// Every method takes the tenant id first (threaded from the authenticated
// Actor by the handler layer) so data never crosses tenant boundaries.

// ---- Material ----

type MaterialService struct{ repo *repository.MaterialRepo }

func NewMaterialService(repo *repository.MaterialRepo) *MaterialService {
	return &MaterialService{repo: repo}
}

func (s *MaterialService) Create(ctx context.Context, t uint, m *model.Material) error {
	if m.SKUCode == "" || m.Name == "" || m.Unit == "" {
		return errorsBadRequest("sku_code/name/unit are required")
	}
	return s.repo.Create(ctx, t, m)
}

// CreateBatch inserts many materials in a single DB transaction (all-or-nothing).
// This is the preferred entry point whenever the user asks for "批量导入 / 批量创建 /
// 批量写入物料" — the agent MUST call this instead of looping material_create N
// times, which would leak IDs on partial failure and waste context budget.
func (s *MaterialService) CreateBatch(ctx context.Context, t uint, list []model.Material) ([]model.Material, error) {
	if len(list) == 0 {
		return nil, errorsBadRequest("items is empty")
	}
	if len(list) > 500 {
		return nil, errorsBadRequest(fmt.Sprintf("batch size exceeds 500: got %d", len(list)))
	}
	// Validate every row up-front so the agent sees a deterministic error
	// report instead of a partial transaction rollback.
	for i := range list {
		m := &list[i]
		if m.SKUCode == "" || m.Name == "" || m.Unit == "" {
			return nil, errorsBadRequest(fmt.Sprintf("sku_code/name/unit are required on every item (index %d)", i))
		}
		if m.Status == 0 {
			m.Status = 1
		}
	}
	if err := s.repo.CreateBatch(ctx, t, list); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *MaterialService) Update(ctx context.Context, t, id uint, m *model.Material) error {
	old, err := s.repo.Get(ctx, t, id)
	if old == nil {
		return errNotFound(id)
	}
	if err != nil {
		return err
	}
	m.ID = id
	if m.CreatedBy == "" {
		m.CreatedBy = old.CreatedBy
	}
	return s.repo.Update(ctx, t, m)
}

func (s *MaterialService) Get(ctx context.Context, t, id uint) (*model.Material, error) {
	return s.repo.Get(ctx, t, id)
}

func (s *MaterialService) Delete(ctx context.Context, t, id uint) error {
	return s.repo.Delete(ctx, t, id)
}

func (s *MaterialService) List(ctx context.Context, t uint, in PageInput) ([]model.Material, int64, error) {
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

func (s *SupplierService) Create(ctx context.Context, t uint, m *model.Supplier) error {
	if m.SupplierCode == "" || m.Name == "" {
		return errorsBadRequest("supplier_code/name are required")
	}
	return s.repo.Create(ctx, t, m)
}

func (s *SupplierService) Update(ctx context.Context, t, id uint, m *model.Supplier) error {
	old, err := s.repo.Get(ctx, t, id)
	if old == nil {
		return errNotFound(id)
	}
	if err != nil {
		return err
	}
	m.ID = id
	if m.CreatedBy == "" {
		m.CreatedBy = old.CreatedBy
	}
	return s.repo.Update(ctx, t, m)
}

func (s *SupplierService) Get(ctx context.Context, t, id uint) (*model.Supplier, error) {
	return s.repo.Get(ctx, t, id)
}

func (s *SupplierService) Delete(ctx context.Context, t, id uint) error {
	return s.repo.Delete(ctx, t, id)
}

func (s *SupplierService) List(ctx context.Context, t uint, in PageInput) ([]model.Supplier, int64, error) {
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
func (s *SupplierService) SetAuditStatus(ctx context.Context, t, id uint, status string) (*model.Supplier, error) {
	if status != model.AuditPending && status != model.AuditApproved && status != model.AuditRejected {
		return nil, errorsBadRequest("audit_status must be PENDING, APPROVED or REJECTED")
	}
	m, err := s.repo.Get(ctx, t, id)
	if m == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	m.AuditStatus = status
	if err := s.repo.Update(ctx, t, m); err != nil {
		return nil, err
	}
	return m, nil
}

// ---- Warehouse ----

type WarehouseService struct{ repo *repository.WarehouseRepo }

func NewWarehouseService(repo *repository.WarehouseRepo) *WarehouseService {
	return &WarehouseService{repo: repo}
}

func (s *WarehouseService) Create(ctx context.Context, t uint, m *model.Warehouse) error {
	if m.WarehouseCode == "" || m.Name == "" {
		return errorsBadRequest("warehouse_code/name are required")
	}
	return s.repo.Create(ctx, t, m)
}

func (s *WarehouseService) Update(ctx context.Context, t, id uint, m *model.Warehouse) error {
	old, err := s.repo.Get(ctx, t, id)
	if old == nil {
		return errNotFound(id)
	}
	if err != nil {
		return err
	}
	m.ID = id
	if m.CreatedBy == "" {
		m.CreatedBy = old.CreatedBy
	}
	return s.repo.Update(ctx, t, m)
}

func (s *WarehouseService) Get(ctx context.Context, t, id uint) (*model.Warehouse, error) {
	return s.repo.Get(ctx, t, id)
}

func (s *WarehouseService) Delete(ctx context.Context, t, id uint) error {
	return s.repo.Delete(ctx, t, id)
}

func (s *WarehouseService) List(ctx context.Context, t uint, in PageInput) ([]model.Warehouse, int64, error) {
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
