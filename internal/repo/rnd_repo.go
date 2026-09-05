package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"scm/internal/model"
)

// ---- Product ----

type ProductRepo struct{ *tenantRepo[model.Product] }

func NewProductRepo(db *gormDB) *ProductRepo {
	return &ProductRepo{tenantRepo: newTenantRepo[model.Product](db)}
}

func (r *ProductRepo) List(f ListFilter, out *[]model.Product, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("product_code LIKE ? OR name LIKE ?", like, like)
		}
		return q.Order("id DESC")
	}
	return r.listT(f, apply, out, total)
}

// ---- BOM ----

type BOMRepo struct {
	*tenantRepo[model.BOM]
	db *gormDB
}

func NewBOMRepo(db *gormDB) *BOMRepo {
	return &BOMRepo{tenantRepo: newTenantRepo[model.BOM](db), db: db}
}

func (r *BOMRepo) GetWithDetails(t, id uint) (*model.BOM, error) {
	var b model.BOM
	if err := r.db.DB.Preload("Details").Where("tenant_id = ?", t).First(&b, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

// CreateWithDetails inserts a BOM and its lines in one transaction.
func (r *BOMRepo) CreateWithDetails(t uint, b *model.BOM) error {
	b.TenantID = t
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(b).Error; err != nil {
			return err
		}
		for i := range b.Details {
			b.Details[i].BOMID = b.ID
			if err := tx.Create(&b.Details[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateWithDetails replaces a BOM header + lines (DRAFT only, enforced by
// the service). Lines are wiped and re-inserted.
func (r *BOMRepo) UpdateWithDetails(t uint, b *model.BOM) error {
	b.TenantID = t
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bom_id = ?", b.ID).Delete(&model.BOMDetail{}).Error; err != nil {
			return err
		}
		for i := range b.Details {
			b.Details[i].ID = 0
			b.Details[i].BOMID = b.ID
			if err := tx.Create(&b.Details[i]).Error; err != nil {
				return err
			}
		}
		saved := b.Details
		b.Details = nil
		err := tx.Where("tenant_id = ?", t).Save(b).Error
		b.Details = saved
		return err
	})
}

func (r *BOMRepo) List(f ListFilter, out *[]model.BOM, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("bom_no LIKE ?", like)
		}
		return q.Order("id DESC")
	}
	return r.listT(f, apply, out, total)
}

// ListByProduct returns all versions of a product's BOMs, newest first.
func (r *BOMRepo) ListByProduct(t, productID uint, out *[]model.BOM) error {
	return r.db.DB.Preload("Details").
		Where("tenant_id = ? AND product_id = ?", t, productID).
		Order("version DESC").
		Find(out).Error
}

// DefaultByProduct loads the effective (RELEASED) BOM of a product.
func (r *BOMRepo) DefaultByProduct(t, productID uint) (*model.BOM, error) {
	var b model.BOM
	if err := r.db.DB.Preload("Details").
		Where("tenant_id = ? AND product_id = ? AND is_default = ? AND status = ?", t, productID, true, model.BOMStatusReleased).
		First(&b).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}

// CountByProduct returns how many BOM versions a product has.
func (r *BOMRepo) CountByProduct(t, productID uint) (int64, error) {
	var n int64
	err := r.db.DB.Model(&model.BOM{}).
		Where("tenant_id = ? AND product_id = ?", t, productID).Count(&n).Error
	return n, err
}

// DemoteDefault flips any current default BOM of a product to OBSOLETE inside
// the caller's transaction (used by Release to make room for the new default).
func (r *BOMRepo) DemoteDefault(tx *gorm.DB, t, productID uint) error {
	return tx.Model(&model.BOM{}).
		Where("tenant_id = ? AND product_id = ? AND is_default = ?", t, productID, true).
		Updates(map[string]any{"is_default": false, "status": model.BOMStatusObsolete}).Error
}
