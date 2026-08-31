package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"scm/internal/model"
	"scm/internal/repository"
)

// ReceivingService closes the purchase loop. One receiving round:
//
//  1. validates against the PO (status, line ownership, over-receipt),
//  2. books ACCEPTED quantities into stock and advances received_qty,
//  3. keeps REJECTED quantities out of stock — they are recorded on the
//     receipt (with a reason) as evidence for supplier return/re-delivery,
//     and stay "on order" for MRP until re-delivered or the PO is closed,
//  4. advances the PO status (IN_PROGRESS, then COMPLETED once every line is
//     fully accepted).
//
// Everything happens in one transaction so receipt, PO progress, stock and
// audit log stay consistent.
type ReceivingService struct {
	receipts *repository.PurchaseReceiptRepo
	pos      *repository.PurchaseOrderRepo
	inv      *InventoryService
	db       *gorm.DB
}

// ReceivingDeps groups the dependencies a ReceivingService needs.
type ReceivingDeps struct {
	Receipts *repository.PurchaseReceiptRepo
	POs      *repository.PurchaseOrderRepo
	Inv      *InventoryService
	DB       *gorm.DB
}

func NewReceivingService(d ReceivingDeps) *ReceivingService {
	return &ReceivingService{
		receipts: d.Receipts,
		pos:      d.POs,
		inv:      d.Inv,
		db:       d.DB,
	}
}

// ReceiveInput describes one receiving round against a PO.
type ReceiveInput struct {
	ReceiptNumber string
	WarehouseID   uint
	ReceiptDate   time.Time
	Remark        string
	Details       []ReceiveDetailInput
}

// ReceiveDetailInput is one receiving line. Passed enters stock; rejected
// stays out of stock. received_qty is derived as passed + rejected.
type ReceiveDetailInput struct {
	PODetailID   uint
	LocationID   uint
	PassedQty    decimal.Decimal
	RejectedQty  decimal.Decimal
	RejectReason string
}

// Receive executes one receiving round and returns the persisted receipt.
func (s *ReceivingService) Receive(t, poID uint, in ReceiveInput) (*model.PurchaseReceipt, error) {
	po, err := s.pos.GetWithDetails(t, poID)
	if po == nil {
		return nil, errNotFound(poID)
	}
	if err != nil {
		return nil, err
	}
	if po.Status != model.POStatusApproved && po.Status != model.POStatusInProgress {
		return nil, errorsBadRequest("only APPROVED or IN_PROGRESS purchase orders can be received")
	}
	if in.WarehouseID == 0 {
		return nil, errorsBadRequest("warehouse_id is required")
	}
	if len(in.Details) == 0 {
		return nil, errorsBadRequest("at least one receiving detail is required")
	}

	byDetail := make(map[uint]*model.PurchaseOrderDetail, len(po.Details))
	for i := range po.Details {
		byDetail[po.Details[i].ID] = &po.Details[i]
	}

	rc := &model.PurchaseReceipt{
		ReceiptNumber: in.ReceiptNumber,
		POID:          poID,
		WarehouseID:   in.WarehouseID,
		ReceiptDate:   in.ReceiptDate,
		Remark:        in.Remark,
	}
	if rc.ReceiptDate.IsZero() {
		rc.ReceiptDate = time.Now()
	}

	// Build receipt lines and the stock movement for the accepted part.
	var moveDetails []MoveDetailInput
	for _, d := range in.Details {
		line, ok := byDetail[d.PODetailID]
		if !ok {
			return nil, errorsBadRequest("po_detail_id " + itob(d.PODetailID) + " does not belong to this purchase order")
		}
		if d.PassedQty.IsNegative() || d.RejectedQty.IsNegative() {
			return nil, errorsBadRequest("passed_qty and rejected_qty must not be negative")
		}
		if d.PassedQty.IsZero() && d.RejectedQty.IsZero() {
			return nil, errorsBadRequest("each receiving line needs a positive passed or rejected quantity")
		}
		if d.RejectedQty.IsPositive() && strings.TrimSpace(d.RejectReason) == "" {
			return nil, errorsBadRequest("reject_reason is required when rejected_qty is positive")
		}
		loc := d.LocationID
		rc.Details = append(rc.Details, model.PurchaseReceiptDetail{
			PODetailID:   line.ID,
			MaterialID:   line.MaterialID,
			LocationID:   loc,
			ReceivedQty:  d.PassedQty.Add(d.RejectedQty),
			PassedQty:    d.PassedQty,
			RejectedQty:  d.RejectedQty,
			RejectReason: d.RejectReason,
		})
		if d.PassedQty.IsPositive() {
			moveDetails = append(moveDetails, MoveDetailInput{
				MaterialID: line.MaterialID,
				LocationID: loc,
				Qty:        d.PassedQty,
			})
		}
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Default receipt number is a per-PO sequence (counted inside the tx
		// so rapid successive receipts never collide on the unique index).
		if rc.ReceiptNumber == "" {
			var count int64
			if err := tx.Model(&model.PurchaseReceipt{}).
				Where("tenant_id = ? AND po_id = ?", t, poID).Count(&count).Error; err != nil {
				return err
			}
			rc.ReceiptNumber = fmt.Sprintf("RCV-%s-%02d", po.PONumber, count+1)
		}
		if err := s.receipts.CreateWithDetailsInTx(tx, t, rc); err != nil {
			return err
		}

		// received_qty accumulates ACCEPTED qty only; the SQL guard rejects
		// over-receipt. Rejected qty deliberately stays out so the on-order
		// qty seen by MRP keeps the rejected amount until re-delivery.
		for _, d := range rc.Details {
			if d.PassedQty.IsZero() {
				continue
			}
			rows, err := s.pos.IncrReceivedQtyInTx(tx, d.PODetailID, d.PassedQty)
			if err != nil {
				return err
			}
			if rows == 0 {
				return errf(ErrBadRequest, "accepted qty exceeds remaining order qty for po detail "+itob(d.PODetailID))
			}
		}

		// Book the accepted quantities into stock (rejected ones never touch
		// inv_stock / the audit log).
		if len(moveDetails) > 0 {
			if _, err := s.inv.applyMovementInTx(tx, t, MoveInput{
				OrderNumber:    "INV-" + rc.ReceiptNumber,
				OrderType:      model.OrderTypePurchaseIn,
				RefOrderNumber: rc.ReceiptNumber,
				WarehouseID:    in.WarehouseID,
				Details:        moveDetails,
			}); err != nil {
				return err
			}
		}

		// Advance the PO status based on the updated accepted quantities.
		details, err := s.pos.ListDetailsInTx(tx, poID)
		if err != nil {
			return err
		}
		next := model.POStatusInProgress
		fully := true
		for _, d := range details {
			if d.ReceivedQty.LessThan(d.OrderQty) {
				fully = false
				break
			}
		}
		if fully {
			next = model.POStatusCompleted
		}
		return tx.Model(&model.PurchaseOrder{}).
			Where("id = ? AND tenant_id = ?", poID, t).
			Update("status", next).Error
	})
	if err != nil {
		return nil, err
	}
	return rc, nil
}

// ListReceipts returns all receiving rounds of a PO, newest first.
func (s *ReceivingService) ListReceipts(t, poID uint) ([]model.PurchaseReceipt, error) {
	var out []model.PurchaseReceipt
	if err := s.receipts.ListByPO(t, poID, &out); err != nil {
		return nil, err
	}
	return out, nil
}
