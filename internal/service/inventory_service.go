package service

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"scm/internal/model"
	"scm/internal/repo"
)

// InventoryService owns inventory-movement business logic. It coordinates the
// stock, inventory-order and transaction-log repositories so a movement stays
// consistent: the order, the on-hand stock and the audit log all move together
// inside one transaction. Every entrypoint takes the tenant id first.
type InventoryService struct {
	stock  *repository.StockRepo
	orders *repository.InventoryOrderRepo
	logs   *repository.TransactionLogRepo
	db     *gorm.DB
}

// InventoryDeps groups the dependencies an InventoryService needs.
type InventoryDeps struct {
	Stock  *repository.StockRepo
	Orders *repository.InventoryOrderRepo
	Logs   *repository.TransactionLogRepo
	DB     *gorm.DB
}

func NewInventoryService(d InventoryDeps) *InventoryService {
	return &InventoryService{
		stock:  d.Stock,
		orders: d.Orders,
		logs:   d.Logs,
		db:     d.DB,
	}
}

// StockService is a thin CRUD facade over inv_stock for the UI.
type StockService struct{ repo *repository.StockRepo }

func NewStockService(repo *repository.StockRepo) *StockService {
	return &StockService{repo: repo}
}

func (s *StockService) List(t uint, in PageInput) ([]model.Stock, int64, error) {
	var (
		out   []model.Stock
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// GetByComposite returns the on-hand stock for a wh/loc/material tuple.
func (s *StockService) GetByComposite(t, wh, loc, mat uint) (*model.Stock, error) {
	return s.repo.GetByComposite(t, wh, loc, mat)
}

// MoveInput describes a single inventory movement (in or out).
type MoveInput struct {
	OrderNumber    string
	OrderType      string
	RefOrderNumber string
	WarehouseID    uint
	Details        []MoveDetailInput
}

// MoveDetailInput is a single movement line.
type MoveDetailInput struct {
	MaterialID uint
	LocationID uint
	Qty        decimal.Decimal
}

// movement runs a movement in its own transaction. It is the standalone
// entrypoint used by MoveIn/MoveOut; composed flows (e.g. purchasing
// receiving) call applyMovementInTx inside their own transaction instead.
func (s *InventoryService) movement(t uint, in MoveInput) (*model.InventoryOrder, error) {
	var created *model.InventoryOrder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		o, err := s.applyMovementInTx(tx, t, in)
		if err != nil {
			return err
		}
		created = o
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.orders.GetWithDetails(t, created.ID)
}

// applyMovementInTx executes a movement inside a caller-owned transaction: it
// writes the inventory order + details, atomically adjusts inv_stock and
// appends the audit log. IN moves add quantity; OUT moves subtract and are
// guarded so on-hand stock can never go negative (anti-over-sell).
func (s *InventoryService) applyMovementInTx(tx *gorm.DB, t uint, in MoveInput) (*model.InventoryOrder, error) {
	isOut := in.OrderType == model.OrderTypeSaleOut

	order := &model.InventoryOrder{
		OrderNumber:    in.OrderNumber,
		OrderType:      in.OrderType,
		RefOrderNumber: in.RefOrderNumber,
		WarehouseID:    in.WarehouseID,
		Status:         model.InvOrderCompleted,
	}
	for _, d := range in.Details {
		if d.Qty.LessThanOrEqual(decimal.Zero) {
			return nil, errorsBadRequest("qty must be positive")
		}
		order.Details = append(order.Details, model.InventoryOrderDetail{
			MaterialID: d.MaterialID,
			LocationID: d.LocationID,
			Qty:        d.Qty,
		})
	}

	if err := s.orders.CreateWithDetailsInTx(tx, t, order); err != nil {
		return nil, err
	}
	for _, d := range in.Details {
		delta := d.Qty
		if isOut {
			delta = d.Qty.Neg()
		}

		cur, err := s.stock.GetByCompositeInTx(tx, t, in.WarehouseID, d.LocationID, d.MaterialID)
		if err != nil {
			return nil, err
		}

		if cur == nil {
			// first-time movement for this tuple: create the stock row.
			after := decimal.Zero
			if !isOut {
				after = delta
			}
			if err := s.stock.UpsertStock(tx, t, &model.Stock{
				WarehouseID: in.WarehouseID,
				LocationID:  d.LocationID,
				MaterialID:  d.MaterialID,
				Quantity:    after,
			}); err != nil {
				return nil, err
			}
			if err := s.logs.Save(tx, t, s.newLog(d, in, model.ActionIn, decimal.Zero, after)); err != nil {
				return nil, err
			}
			continue
		}

		// Guarded atomic update. For OUT we add the WHERE quantity>=X
		// predicate so a concurrent read can never oversell.
		rows, err := s.stock.AdjustGated(tx, t, in.WarehouseID, d.LocationID, d.MaterialID, delta, isOut, d.Qty)
		if err != nil {
			return nil, err
		}
		if rows == 0 {
			return nil, errf(ErrBadRequest, "insufficient on-hand stock for material "+itob(d.MaterialID))
		}
		before := cur.Quantity
		after := before.Add(delta)
		if err := s.logs.Save(tx, t, s.newLog(d, in, logAction(in.OrderType), before, after)); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// MoveIn handles purchase-in (and transfer-in) movements.
func (s *InventoryService) MoveIn(t uint, in MoveInput) (*model.InventoryOrder, error) {
	return s.movement(t, in)
}

// MoveOut handles sale-out movements (oversell guarded).
func (s *InventoryService) MoveOut(t uint, in MoveInput) (*model.InventoryOrder, error) {
	return s.movement(t, in)
}

func (s *InventoryService) newLog(d MoveDetailInput, in MoveInput, action string, before, after decimal.Decimal) *model.TransactionLog {
	return &model.TransactionLog{
		MaterialID:     d.MaterialID,
		WarehouseID:    in.WarehouseID,
		LocationID:     d.LocationID,
		ActionType:     action,
		ChangeQty:      after.Sub(before),
		BeforeQty:      before,
		AfterQty:       after,
		RefOrderNumber: in.OrderNumber,
	}
}

func logAction(orderType string) string {
	if orderType == model.OrderTypeSaleOut {
		return model.ActionOut
	}
	return model.ActionIn
}

// ListOrders returns paginated inventory orders within the tenant.
func (s *InventoryService) ListOrders(t uint, in PageInput) ([]model.InventoryOrder, int64, error) {
	var (
		out   []model.InventoryOrder
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.orders.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// GetOrder loads a movement with details.
func (s *InventoryService) GetOrder(t, id uint) (*model.InventoryOrder, error) {
	return s.orders.GetWithDetails(t, id)
}

// DeleteOrder removes a movement record.
func (s *InventoryService) DeleteOrder(t, id uint) error {
	return s.orders.Delete(t, id)
}

// ListLogs returns the audit trail within the tenant.
func (s *InventoryService) ListLogs(t uint, in PageInput) ([]model.TransactionLog, int64, error) {
	var (
		out   []model.TransactionLog
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.logs.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
