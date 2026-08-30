package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/shopspring/decimal"

	"scm/internal/model"
)

// PurchaseOrderRepo owns pur_order + pur_order_detail.
type PurchaseOrderRepo struct {
	*genericRepo[model.PurchaseOrder]
	db *gormDB
}

func NewPurchaseOrderRepo(db *gormDB) *PurchaseOrderRepo {
	return &PurchaseOrderRepo{
		genericRepo: newGenericRepo[model.PurchaseOrder](db),
		db:          db,
	}
}

// GetWithDetails loads a PO with its details preloaded.
func (r *PurchaseOrderRepo) GetWithDetails(id uint) (*model.PurchaseOrder, error) {
	var po model.PurchaseOrder
	if err := r.db.DB.Preload("Details").First(&po, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &po, nil
}

// CreateWithDetails inserts a PO and its details in one transaction. The
// header is created with associations omitted so the explicit detail loop
// below stays the single writer (GORM would otherwise cascade-insert the
// Details slice and collide on primary keys).
func (r *PurchaseOrderRepo) CreateWithDetails(po *model.PurchaseOrder) error {
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(po).Error; err != nil {
			return err
		}
		for i := range po.Details {
			po.Details[i].POID = po.ID
			if err := tx.Create(&po.Details[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateWithDetails replaces a PO header and its detail lines in one
// transaction. Existing lines are wiped first, so an edit is a full replace.
func (r *PurchaseOrderRepo) UpdateWithDetails(po *model.PurchaseOrder) error {
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		// 1) drop the old lines for this PO.
		if err := tx.Where("po_id = ?", po.ID).Delete(&model.PurchaseOrderDetail{}).Error; err != nil {
			return err
		}
		// 2) insert the new lines.
		for i := range po.Details {
			po.Details[i].ID = 0
			po.Details[i].POID = po.ID
			if err := tx.Create(&po.Details[i]).Error; err != nil {
				return err
			}
		}
		// 3) persist the header WITHOUT cascading the details slice (which we
		//    just wrote explicitly), to avoid a second insert.
		saved := po.Details
		po.Details = nil
		err := tx.Save(po).Error
		po.Details = saved
		return err
	})
}

// UpdateColumns applies a selective column update to the PO header only —
// details, totals, status and received progress stay untouched.
func (r *PurchaseOrderRepo) UpdateColumns(id uint, cols map[string]any) error {
	return r.db.DB.Model(&model.PurchaseOrder{}).
		Where("id = ?", id).
		Updates(cols).Error
}

// List returns paginated POs ordered newest first.
func (r *PurchaseOrderRepo) List(f ListFilter, out *[]model.PurchaseOrder, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("po_number LIKE ?", like)
		}
		return q.Order("id DESC")
	}
	return r.list(f, apply, out, total)
}

// UpdateDetail persists a single detail line (e.g. received_qty updates).
func (r *PurchaseOrderRepo) UpdateDetail(d *model.PurchaseOrderDetail) error {
	return r.db.DB.Save(d).Error
}

// IncrReceivedQtyInTx adds inc to a detail's received_qty inside a caller
// transaction. The SQL predicate guards against over-receipt
// (received_qty + inc must not exceed order_qty); 0 affected rows means the
// guard rejected the update.
func (r *PurchaseOrderRepo) IncrReceivedQtyInTx(tx *gorm.DB, detailID uint, inc decimal.Decimal) (int64, error) {
	res := tx.Model(&model.PurchaseOrderDetail{}).
		Where("id = ? AND received_qty + ? <= order_qty", detailID, inc).
		Update("received_qty", gorm.Expr("received_qty + ?", inc))
	return res.RowsAffected, res.Error
}

// ListDetailsInTx loads the detail lines of a PO inside a caller transaction.
func (r *PurchaseOrderRepo) ListDetailsInTx(tx *gorm.DB, poID uint) ([]model.PurchaseOrderDetail, error) {
	var out []model.PurchaseOrderDetail
	err := tx.Where("po_id = ?", poID).Find(&out).Error
	return out, err
}

// PurchaseReceiptRepo owns pur_receipt + pur_receipt_detail.
type PurchaseReceiptRepo struct {
	*genericRepo[model.PurchaseReceipt]
	db *gormDB
}

func NewPurchaseReceiptRepo(db *gormDB) *PurchaseReceiptRepo {
	return &PurchaseReceiptRepo{
		genericRepo: newGenericRepo[model.PurchaseReceipt](db),
		db:          db,
	}
}

// CreateWithDetailsInTx persists a receipt and its lines inside an outer tx.
func (r *PurchaseReceiptRepo) CreateWithDetailsInTx(tx *gorm.DB, rc *model.PurchaseReceipt) error {
	if err := tx.Omit(clause.Associations).Create(rc).Error; err != nil {
		return err
	}
	for i := range rc.Details {
		rc.Details[i].ReceiptID = rc.ID
		if err := tx.Create(&rc.Details[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// ListByPO returns all receipts (with details) of one PO, newest first.
func (r *PurchaseReceiptRepo) ListByPO(poID uint, out *[]model.PurchaseReceipt) error {
	return r.db.DB.Preload("Details").
		Where("po_id = ?", poID).
		Order("id DESC").
		Find(out).Error
}
