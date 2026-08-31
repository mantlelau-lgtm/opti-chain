package service

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"scm/internal/model"
	"scm/internal/repository"
)

// ---- Product ----

// ProductService owns R&D product master data.
type ProductService struct{ repo *repository.ProductRepo }

func NewProductService(repo *repository.ProductRepo) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) Create(t uint, m *model.Product) error {
	if m.ProductCode == "" || m.Name == "" || m.Unit == "" {
		return errorsBadRequest("product_code/name/unit are required")
	}
	return s.repo.Create(t, m)
}

func (s *ProductService) Update(t, id uint, m *model.Product) error {
	m.ID = id
	return s.repo.Update(t, m)
}

func (s *ProductService) Get(t, id uint) (*model.Product, error) {
	return s.repo.Get(t, id)
}

func (s *ProductService) Delete(t, id uint) error {
	return s.repo.Delete(t, id)
}

func (s *ProductService) List(t uint, in PageInput) ([]model.Product, int64, error) {
	var (
		out   []model.Product
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ---- BOM ----

// BOMService owns the versioned bill-of-materials lifecycle:
//
//	DRAFT --release--> RELEASED (becomes the effective/default version)
//	editing is only allowed in DRAFT; a released BOM is changed by opening a
//	new version.
type BOMService struct {
	repo      *repository.BOMRepo
	products  *repository.ProductRepo
	materials *repository.MaterialRepo
	db        *gorm.DB
}

// BOMDeps groups the dependencies a BOMService needs.
type BOMDeps struct {
	Repo      *repository.BOMRepo
	Products  *repository.ProductRepo
	Materials *repository.MaterialRepo
	DB        *gorm.DB
}

func NewBOMService(d BOMDeps) *BOMService {
	return &BOMService{repo: d.Repo, products: d.Products, materials: d.Materials, db: d.DB}
}

// BOMDetailInput is one component line.
type BOMDetailInput struct {
	ComponentID uint
	QtyPerUnit  decimal.Decimal
	ScrapRate   decimal.Decimal
	Remark      string
}

// BOMInput is the payload for creating/updating a BOM.
type BOMInput struct {
	BOMNo     string
	ProductID uint
	UnitQty   decimal.Decimal
	Remark    string
	Details   []BOMDetailInput
}

// Create validates and persists a DRAFT BOM. The version is auto-assigned as
// max(existing versions) + 1 for the product.
func (s *BOMService) Create(t uint, in BOMInput) (*model.BOM, error) {
	if in.ProductID == 0 || len(in.Details) == 0 {
		return nil, errorsBadRequest("product_id and at least one component are required")
	}
	if err := s.validateDetails(t, in.Details); err != nil {
		return nil, err
	}
	if _, err := s.products.Get(t, in.ProductID); err != nil {
		return nil, errNotFound(in.ProductID)
	}
	version := int64(1)
	if n, _ := s.repo.CountByProduct(t, in.ProductID); n > 0 {
		version = n + 1
	}
	b := &model.BOM{
		BOMNo:     in.BOMNo,
		ProductID: in.ProductID,
		Version:   int(version),
		Status:    model.BOMStatusDraft,
		UnitQty:   defaultDecimal(in.UnitQty),
		Remark:    in.Remark,
		Details:   toBOMDetails(in.Details),
	}
	if err := s.repo.CreateWithDetails(t, b); err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(t, b.ID)
}

// Update replaces a DRAFT BOM's header + lines. Released BOMs are immutable.
func (s *BOMService) Update(t, id uint, in BOMInput) (*model.BOM, error) {
	b, err := s.repo.GetWithDetails(t, id)
	if b == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	if b.Status != model.BOMStatusDraft {
		return nil, errorsBadRequest("only DRAFT BOMs can be edited; open a new version instead")
	}
	if len(in.Details) == 0 {
		return nil, errorsBadRequest("at least one component is required")
	}
	if err := s.validateDetails(t, in.Details); err != nil {
		return nil, err
	}
	b.BOMNo = in.BOMNo
	b.UnitQty = defaultDecimal(in.UnitQty)
	b.Remark = in.Remark
	b.Details = toBOMDetails(in.Details)
	if err := s.repo.UpdateWithDetails(t, b); err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(t, id)
}

// Delete removes a DRAFT BOM.
func (s *BOMService) Delete(t, id uint) error {
	b, err := s.repo.Get(t, id)
	if b == nil {
		return errNotFound(id)
	}
	if err != nil {
		return err
	}
	if b.Status != model.BOMStatusDraft {
		return errorsBadRequest("only DRAFT BOMs can be deleted")
	}
	return s.repo.Delete(t, id)
}

// Release promotes a DRAFT BOM to RELEASED and makes it the product's default,
// demoting the previous default (if any) to OBSOLETE, in one transaction.
func (s *BOMService) Release(t, id uint) (*model.BOM, error) {
	b, err := s.repo.GetWithDetails(t, id)
	if b == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	if b.Status != model.BOMStatusDraft {
		return nil, errorsBadRequest("only DRAFT BOMs can be released")
	}
	if len(b.Details) == 0 {
		return nil, errorsBadRequest("cannot release an empty BOM")
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.DemoteDefault(tx, t, b.ProductID); err != nil {
			return err
		}
		return tx.Model(&model.BOM{}).
			Where("id = ? AND tenant_id = ?", b.ID, t).
			Updates(map[string]any{"status": model.BOMStatusReleased, "is_default": true}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(t, id)
}

func (s *BOMService) Get(t, id uint) (*model.BOM, error) {
	return s.repo.GetWithDetails(t, id)
}

func (s *BOMService) List(t uint, in PageInput) ([]model.BOM, int64, error) {
	var (
		out   []model.BOM
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *BOMService) ListByProduct(t, productID uint) ([]model.BOM, error) {
	var out []model.BOM
	if err := s.repo.ListByProduct(t, productID, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DefaultByProduct returns the effective BOM of a product (nil when none).
func (s *BOMService) DefaultByProduct(t, productID uint) (*model.BOM, error) {
	return s.repo.DefaultByProduct(t, productID)
}

func (s *BOMService) validateDetails(t uint, ds []BOMDetailInput) error {
	for _, d := range ds {
		if d.QtyPerUnit.LessThanOrEqual(decimal.Zero) {
			return errorsBadRequest("component qty must be positive")
		}
		if m, _ := s.materials.Get(t, d.ComponentID); m == nil {
			return errNotFound(d.ComponentID)
		}
	}
	return nil
}

func toBOMDetails(ds []BOMDetailInput) []model.BOMDetail {
	out := make([]model.BOMDetail, 0, len(ds))
	for _, d := range ds {
		out = append(out, model.BOMDetail{
			ComponentID: d.ComponentID,
			QtyPerUnit:  d.QtyPerUnit,
			ScrapRate:   d.ScrapRate,
			Remark:      d.Remark,
		})
	}
	return out
}

func defaultDecimal(d decimal.Decimal) decimal.Decimal {
	if d.IsZero() {
		return decimal.NewFromInt(1)
	}
	return d
}
