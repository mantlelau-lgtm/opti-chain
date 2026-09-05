package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/shopspring/decimal"

	"scm/internal/model"
)

// StockRepo owns the real-time inv_stock table.
type StockRepo struct{ *tenantRepo[model.Stock] }

func NewStockRepo(db *gormDB) *StockRepo {
	return &StockRepo{tenantRepo: newTenantRepo[model.Stock](db)}
}

func (r *StockRepo) List(f ListFilter, out *[]model.Stock, total *int64) error {
	return r.listT(f, func(q *gorm.DB) *gorm.DB { return q.Order("id DESC") }, out, total)
}

// GetByComposite fetches a stock row by (tenant, warehouse, location, material).
func (r *StockRepo) GetByComposite(t, wh, loc, mat uint) (*model.Stock, error) {
	var s model.Stock
	err := r.db.DB.Where("tenant_id = ? AND warehouse_id = ? AND location_id = ? AND material_id = ?", t, wh, loc, mat).
		First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Adjust atomically applies delta to quantity and returns rows affected.
// For OUT movements delta is negative. This is the guard against negative
// stock / overselling.
func (r *StockRepo) Adjust(tx *gorm.DB, t, wh, loc, mat uint, delta decimal.Decimal) (int64, error) {
	res := tx.Model(&model.Stock{}).
		Where("tenant_id = ? AND warehouse_id = ? AND location_id = ? AND material_id = ?", t, wh, loc, mat).
		Update("quantity", gorm.Expr("quantity + ?", delta))
	return res.RowsAffected, res.Error
}

// UpsertStock ensures a stock row exists (creating at 0 if missing).
func (r *StockRepo) UpsertStock(tx *gorm.DB, t uint, s *model.Stock) error {
	s.TenantID = t
	var existing model.Stock
	err := tx.Where("tenant_id = ? AND warehouse_id = ? AND location_id = ? AND material_id = ?",
		t, s.WarehouseID, s.LocationID, s.MaterialID).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return tx.Create(s).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&existing).Update("quantity", s.Quantity).Error
}

// GetByCompositeInTx fetches a stock row within a transaction.
func (r *StockRepo) GetByCompositeInTx(tx *gorm.DB, t, wh, loc, mat uint) (*model.Stock, error) {
	var s model.Stock
	err := tx.Where("tenant_id = ? AND warehouse_id = ? AND location_id = ? AND material_id = ?", t, wh, loc, mat).
		First(&s).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// AdjustGated atomically applies delta to a stock row. When guard is true it
// also enforces quantity + delta >= 0, preventing overselling even under
// concurrency. Returns rows affected (0 means the guard rejected the update).
func (r *StockRepo) AdjustGated(tx *gorm.DB, t, wh, loc, mat uint, delta decimal.Decimal, guard bool, floor decimal.Decimal) (int64, error) {
	q := tx.Model(&model.Stock{}).
		Where("tenant_id = ? AND warehouse_id = ? AND location_id = ? AND material_id = ?", t, wh, loc, mat)
	if guard {
		q = q.Where("quantity + ? >= 0", delta)
	}
	res := q.Update("quantity", gorm.Expr("quantity + ?", delta))
	return res.RowsAffected, res.Error
}

// AvailableRowsInTx lists a material's stock rows with positive available qty
// (quantity - locked_quantity), largest first, inside a caller transaction.
// Used to allocate sales reservations.
func (r *StockRepo) AvailableRowsInTx(tx *gorm.DB, t, matID uint) ([]model.Stock, error) {
	var rows []model.Stock
	err := tx.Where("tenant_id = ? AND material_id = ? AND quantity > locked_quantity", t, matID).
		Order("(quantity - locked_quantity) DESC, id").
		Find(&rows).Error
	return rows, err
}

// LockRowInTx atomically reserves qty on one stock row. The SQL predicate
// enforces available >= qty so a concurrent reservation can never over-lock;
// 0 affected rows means the guard rejected the update.
//
// The guard is written as `quantity >= locked_quantity + ?` (not
// `quantity - locked_quantity >= ?`) so the compared side is a COLUMN: SQLite
// applies NUMERIC affinity to the bound decimal, whereas an expression has no
// affinity and would be compared against the TEXT-encoded decimal.
func (r *StockRepo) LockRowInTx(tx *gorm.DB, stockID uint, qty decimal.Decimal) (int64, error) {
	res := tx.Model(&model.Stock{}).
		Where("id = ? AND quantity >= locked_quantity + ?", stockID, qty).
		Update("locked_quantity", gorm.Expr("locked_quantity + ?", qty))
	return res.RowsAffected, res.Error
}

// UnlockRowInTx releases a reservation on one stock row, guarded so
// locked_quantity never goes negative.
func (r *StockRepo) UnlockRowInTx(tx *gorm.DB, stockID uint, qty decimal.Decimal) (int64, error) {
	res := tx.Model(&model.Stock{}).
		Where("id = ? AND locked_quantity >= ?", stockID, qty).
		Update("locked_quantity", gorm.Expr("locked_quantity - ?", qty))
	return res.RowsAffected, res.Error
}

// LockedRowsInTx lists a material's stock rows carrying reservations, used to
// release them on cancellation.
func (r *StockRepo) LockedRowsInTx(tx *gorm.DB, t, matID uint) ([]model.Stock, error) {
	var rows []model.Stock
	err := tx.Where("tenant_id = ? AND material_id = ? AND locked_quantity > 0", t, matID).
		Order("id").
		Find(&rows).Error
	return rows, err
}

// SumByMaterial returns total on-hand quantity for a material across all
// warehouses/locations of one tenant.
func (r *StockRepo) SumByMaterial(t, matID uint) (decimal.Decimal, error) {
	var res struct {
		Qty decimal.Decimal
	}
	err := r.db.DB.Raw(
		"SELECT COALESCE(SUM(quantity), 0) AS qty FROM inv_stock WHERE tenant_id = ? AND material_id = ?",
		t, matID,
	).Scan(&res).Error
	return res.Qty, err
}

// InventoryOrderRepo owns inv_order + inv_order_detail.
type InventoryOrderRepo struct {
	*tenantRepo[model.InventoryOrder]
	db *gormDB
}

func NewInventoryOrderRepo(db *gormDB) *InventoryOrderRepo {
	return &InventoryOrderRepo{
		tenantRepo: newTenantRepo[model.InventoryOrder](db),
		db:         db,
	}
}

func (r *InventoryOrderRepo) GetWithDetails(t, id uint) (*model.InventoryOrder, error) {
	var o model.InventoryOrder
	if err := r.db.DB.Preload("Details").Where("tenant_id = ?", t).First(&o, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

func (r *InventoryOrderRepo) CreateWithDetails(t uint, o *model.InventoryOrder) error {
	o.TenantID = t
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(o).Error; err != nil {
			return err
		}
		for i := range o.Details {
			o.Details[i].InvOrderID = o.ID
			if err := tx.Create(&o.Details[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// CreateWithDetailsInTx persists an order + details inside an outer tx.
func (r *InventoryOrderRepo) CreateWithDetailsInTx(tx *gorm.DB, t uint, o *model.InventoryOrder) error {
	o.TenantID = t
	if err := tx.Omit(clause.Associations).Create(o).Error; err != nil {
		return err
	}
	for i := range o.Details {
		o.Details[i].InvOrderID = o.ID
		if err := tx.Create(&o.Details[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *InventoryOrderRepo) List(f ListFilter, out *[]model.InventoryOrder, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("order_number LIKE ? OR ref_order_number LIKE ?", like, like)
		}
		return q.Order("id DESC")
	}
	return r.listT(f, apply, out, total)
}

// TransactionLogRepo owns the audit trail.
type TransactionLogRepo struct {
	*tenantRepo[model.TransactionLog]
}

func NewTransactionLogRepo(db *gormDB) *TransactionLogRepo {
	return &TransactionLogRepo{tenantRepo: newTenantRepo[model.TransactionLog](db)}
}

func (r *TransactionLogRepo) List(f ListFilter, out *[]model.TransactionLog, total *int64) error {
	return r.listT(f, func(q *gorm.DB) *gorm.DB { return q.Order("id DESC") }, out, total)
}

// Save persists a single transaction-log row within a caller transaction.
func (r *TransactionLogRepo) Save(tx *gorm.DB, t uint, log *model.TransactionLog) error {
	log.TenantID = t
	return tx.Create(log).Error
}
