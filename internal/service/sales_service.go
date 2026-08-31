package service

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"scm/internal/model"
	"scm/internal/repository"
)

// ---- Customer ----

// CustomerService wraps CustomerRepo with validation.
type CustomerService struct{ repo *repository.CustomerRepo }

func NewCustomerService(repo *repository.CustomerRepo) *CustomerService {
	return &CustomerService{repo: repo}
}

func (s *CustomerService) Create(t uint, m *model.Customer) error {
	if m.CustomerCode == "" || m.Name == "" {
		return errorsBadRequest("customer_code/name are required")
	}
	return s.repo.Create(t, m)
}

func (s *CustomerService) Update(t, id uint, m *model.Customer) error {
	old, err := s.repo.Get(t, id)
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
	return s.repo.Update(t, m)
}

func (s *CustomerService) Get(t, id uint) (*model.Customer, error) {
	return s.repo.Get(t, id)
}

func (s *CustomerService) Delete(t, id uint) error {
	return s.repo.Delete(t, id)
}

func (s *CustomerService) List(t uint, in PageInput) ([]model.Customer, int64, error) {
	var (
		out   []model.Customer
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ---- Sale order ----

// SalesOrderService owns the sales-order lifecycle:
//
//	DRAFT --approve--> APPROVED   (locks available stock, consumes credit)
//	DRAFT/APPROVED --cancel--> CANCELLED  (releases locks + credit)
//
// Stock reservation uses atomic guarded UPDATEs (never SELECT-then-UPDATE),
// so concurrent approvals can never oversell.
type SalesOrderService struct {
	repo      *repository.SaleOrderRepo
	customers *repository.CustomerRepo
	stock     *repository.StockRepo
	db        *gorm.DB
}

// SalesOrderDeps groups the dependencies a SalesOrderService needs.
type SalesOrderDeps struct {
	Repo      *repository.SaleOrderRepo
	Customers *repository.CustomerRepo
	Stock     *repository.StockRepo
	DB        *gorm.DB
}

func NewSalesOrderService(d SalesOrderDeps) *SalesOrderService {
	return &SalesOrderService{repo: d.Repo, customers: d.Customers, stock: d.Stock, db: d.DB}
}

// CreateSOInput is the request payload for creating a sales order.
type CreateSOInput struct {
	SONumber   string
	CustomerID uint
	OrderDate  time.Time
	CreatedBy  string
	Details    []SODetailInput
}

// SODetailInput is a single SO line.
type SODetailInput struct {
	MaterialID uint
	Qty        decimal.Decimal
	UnitPrice  decimal.Decimal
}

// Create validates an SO, computes totals and persists it as DRAFT. The
// customer must exist and be approved (SOP 准入管控).
func (s *SalesOrderService) Create(t uint, in CreateSOInput) (*model.SaleOrder, error) {
	if in.CustomerID == 0 || len(in.Details) == 0 {
		return nil, errorsBadRequest("customer_id and at least one detail are required")
	}
	cust, err := s.customers.Get(t, in.CustomerID)
	if cust == nil {
		return nil, errNotFound(in.CustomerID)
	}
	if err != nil {
		return nil, err
	}
	if cust.AuditStatus != model.AuditApproved {
		return nil, errorsBadRequest("customer is not approved (audit_status must be APPROVED)")
	}

	so := &model.SaleOrder{
		SONumber:    in.SONumber,
		CustomerID:  in.CustomerID,
		OrderDate:   in.OrderDate,
		Status:      model.SOStatusDraft,
		CreatedBy:   in.CreatedBy,
		TotalAmount: decimal.Zero,
	}
	for _, d := range in.Details {
		if d.Qty.LessThanOrEqual(decimal.Zero) {
			return nil, errorsBadRequest("qty must be positive")
		}
		lineTotal := d.Qty.Mul(d.UnitPrice)
		so.Details = append(so.Details, model.SaleOrderDetail{
			MaterialID: d.MaterialID,
			Qty:        d.Qty,
			UnitPrice:  d.UnitPrice,
			TotalPrice: lineTotal,
		})
		so.TotalAmount = so.TotalAmount.Add(lineTotal)
	}
	if err := s.repo.CreateWithDetails(t, so); err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(t, so.ID)
}

func (s *SalesOrderService) Get(t, id uint) (*model.SaleOrder, error) {
	return s.repo.GetWithDetails(t, id)
}

func (s *SalesOrderService) List(t uint, in PageInput) ([]model.SaleOrder, int64, error) {
	var (
		out   []model.SaleOrder
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// Delete removes an SO. Only DRAFT orders may be deleted — approved orders
// hold stock reservations and credit, so they must be cancelled instead.
func (s *SalesOrderService) Delete(t, id uint) error {
	so, err := s.repo.Get(t, id)
	if so == nil {
		return errNotFound(id)
	}
	if err != nil {
		return err
	}
	if so.Status != model.SOStatusDraft {
		return errorsBadRequest("only DRAFT sales orders can be deleted; cancel it instead")
	}
	return s.repo.Delete(t, id)
}

// Approve transitions DRAFT -> APPROVED: every line reserves available stock
// (available = quantity - locked_quantity) and the order amount is charged to
// the customer's credit line. Both happen in one transaction.
func (s *SalesOrderService) Approve(t, id uint) (*model.SaleOrder, error) {
	so, err := s.repo.GetWithDetails(t, id)
	if so == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	if so.Status != model.SOStatusDraft {
		return nil, errorsBadRequest("only DRAFT sales orders can be approved")
	}
	cust, err := s.customers.Get(t, so.CustomerID)
	if cust == nil || err != nil {
		return nil, errNotFound(so.CustomerID)
	}
	// credit_limit <= 0 disables credit control.
	if cust.CreditLimit.IsPositive() &&
		cust.UsedCredit.Add(so.TotalAmount).GreaterThan(cust.CreditLimit) {
		return nil, errf(ErrConflict, "credit limit exceeded: used "+
			cust.UsedCredit.String()+" + order "+so.TotalAmount.String()+
			" > limit "+cust.CreditLimit.String())
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		for _, d := range so.Details {
			if err := s.lockStockTx(tx, t, d.MaterialID, d.Qty); err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Customer{}).
			Where("id = ? AND tenant_id = ?", cust.ID, t).
			Update("used_credit", gorm.Expr("used_credit + ?", so.TotalAmount)).Error; err != nil {
			return err
		}
		res := tx.Model(&model.SaleOrder{}).
			Where("id = ? AND tenant_id = ? AND status = ?", id, t, model.SOStatusDraft).
			Update("status", model.SOStatusApproved)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errf(ErrConflict, "sales order status changed concurrently")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(t, id)
}

// Cancel transitions DRAFT/APPROVED -> CANCELLED. Approved orders release
// their stock reservations and credit.
func (s *SalesOrderService) Cancel(t, id uint) (*model.SaleOrder, error) {
	so, err := s.repo.GetWithDetails(t, id)
	if so == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	if so.Status != model.SOStatusDraft && so.Status != model.SOStatusApproved {
		return nil, errorsBadRequest("only DRAFT or APPROVED sales orders can be cancelled")
	}
	wasApproved := so.Status == model.SOStatusApproved

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if wasApproved {
			for _, d := range so.Details {
				if err := s.unlockStockTx(tx, t, d.MaterialID, d.Qty); err != nil {
					return err
				}
			}
			if err := tx.Model(&model.Customer{}).
				Where("id = ? AND tenant_id = ? AND used_credit >= ?", so.CustomerID, t, so.TotalAmount).
				Update("used_credit", gorm.Expr("used_credit - ?", so.TotalAmount)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.SaleOrder{}).
			Where("id = ? AND tenant_id = ?", id, t).
			Update("status", model.SOStatusCancelled).Error
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(t, id)
}

// lockStockTx reserves qty for a material across stock rows inside tx.
// Allocation is largest-first; every UPDATE is guarded so a concurrent
// reservation can never over-lock (0 rows -> rollback).
func (s *SalesOrderService) lockStockTx(tx *gorm.DB, t, matID uint, qty decimal.Decimal) error {
	rows, err := s.stock.AvailableRowsInTx(tx, t, matID)
	if err != nil {
		return err
	}
	remaining := qty
	for _, r := range rows {
		if !remaining.IsPositive() {
			break
		}
		take := r.Quantity.Sub(r.LockedQuantity)
		if take.GreaterThan(remaining) {
			take = remaining
		}
		n, err := s.stock.LockRowInTx(tx, r.ID, take)
		if err != nil {
			return err
		}
		if n == 0 {
			return errf(ErrConflict, "insufficient available stock for material "+itob(matID))
		}
		remaining = remaining.Sub(take)
	}
	if remaining.IsPositive() {
		return errorsBadRequest("insufficient available stock for material " + itob(matID))
	}
	return nil
}

// unlockStockTx releases a reservation. Locked units are fungible, so the
// release is taken from any rows of the material carrying reservations.
func (s *SalesOrderService) unlockStockTx(tx *gorm.DB, t, matID uint, qty decimal.Decimal) error {
	rows, err := s.stock.LockedRowsInTx(tx, t, matID)
	if err != nil {
		return err
	}
	remaining := qty
	for _, r := range rows {
		if !remaining.IsPositive() {
			break
		}
		take := r.LockedQuantity
		if take.GreaterThan(remaining) {
			take = remaining
		}
		n, err := s.stock.UnlockRowInTx(tx, r.ID, take)
		if err != nil {
			return err
		}
		if n == 0 {
			return errf(ErrConflict, "failed to release stock reservation for material "+itob(matID))
		}
		remaining = remaining.Sub(take)
	}
	if remaining.IsPositive() {
		return errf(ErrConflict, "reservation to release exceeds locked qty for material "+itob(matID))
	}
	return nil
}
