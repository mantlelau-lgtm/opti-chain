package service

import (
	"time"

	"github.com/shopspring/decimal"

	"scm/internal/model"
	"scm/internal/repository"
)

// PurchaseOrderService owns purchase-order lifecycle logic.
type PurchaseOrderService struct {
	repo      *repository.PurchaseOrderRepo
	suppliers *repository.SupplierRepo
}

func NewPurchaseOrderService(repo *repository.PurchaseOrderRepo, suppliers *repository.SupplierRepo) *PurchaseOrderService {
	return &PurchaseOrderService{repo: repo, suppliers: suppliers}
}

// CreateInput is the request payload for creating a PO.
type CreatePOInput struct {
	PONumber             string
	SupplierID           uint
	OrderDate            time.Time
	ExpectedDeliveryDate *time.Time
	CreatedBy            string
	Details              []PODetailInput
}

// PODetailInput is a single PO line.
type PODetailInput struct {
	MaterialID uint
	OrderQty   decimal.Decimal
	UnitPrice  decimal.Decimal
	LocationID uint
}

// Create validates a PO, computes line/total amounts and persists it. The
// supplier must exist and be approved (SOP 准入管控).
func (s *PurchaseOrderService) Create(in CreatePOInput) (*model.PurchaseOrder, error) {
	if in.SupplierID == 0 || len(in.Details) == 0 {
		return nil, errorsBadRequest("supplier_id and at least one detail are required")
	}
	supplier, err := s.suppliers.Get(in.SupplierID)
	if supplier == nil {
		return nil, errNotFound(in.SupplierID)
	}
	if err != nil {
		return nil, err
	}
	if supplier.AuditStatus != model.AuditApproved {
		return nil, errorsBadRequest("supplier is not approved (audit_status must be APPROVED)")
	}
	po := &model.PurchaseOrder{
		PONumber:         in.PONumber,
		SupplierID:       in.SupplierID,
		OrderDate:        in.OrderDate,
		ExpectedDelivery: in.ExpectedDeliveryDate,
		Status:           model.POStatusDraft,
		CreatedBy:        in.CreatedBy,
		TotalAmount:      decimal.Zero,
	}
	for _, d := range in.Details {
		if d.OrderQty.LessThanOrEqual(decimal.Zero) {
			return nil, errorsBadRequest("order_qty must be positive")
		}
		lineTotal := d.OrderQty.Mul(d.UnitPrice)
		po.Details = append(po.Details, model.PurchaseOrderDetail{
			MaterialID:  d.MaterialID,
			OrderQty:    d.OrderQty,
			UnitPrice:   d.UnitPrice,
			ReceivedQty: decimal.Zero,
			TotalPrice:  lineTotal,
		})
		po.TotalAmount = po.TotalAmount.Add(lineTotal)
	}
	if err := s.repo.CreateWithDetails(po); err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(po.ID)
}

// Update replaces a PO header (status transitions handled separately).
func (s *PurchaseOrderService) Update(id uint, po *model.PurchaseOrder) error {
	po.ID = id
	return s.repo.Update(po)
}

// UpdateHeader edits only the header fields supplied by the caller; details,
// totals, status and received progress are never touched (so editing a
// partially received PO cannot wipe its fulfillment state).
func (s *PurchaseOrderService) UpdateHeader(id uint, in CreatePOInput) (*model.PurchaseOrder, error) {
	po, err := s.repo.Get(id)
	if po == nil {
		return nil, errNotFound(id)
	}
	if err != nil {
		return nil, err
	}
	cols := map[string]any{}
	if in.PONumber != "" && in.PONumber != po.PONumber {
		cols["po_number"] = in.PONumber
	}
	if in.SupplierID != 0 && in.SupplierID != po.SupplierID {
		supplier, err := s.suppliers.Get(in.SupplierID)
		if supplier == nil {
			return nil, errNotFound(in.SupplierID)
		}
		if err != nil {
			return nil, err
		}
		if supplier.AuditStatus != model.AuditApproved {
			return nil, errorsBadRequest("supplier is not approved (audit_status must be APPROVED)")
		}
		cols["supplier_id"] = in.SupplierID
	}
	if !in.OrderDate.IsZero() {
		cols["order_date"] = in.OrderDate
	}
	cols["expected_delivery_date"] = in.ExpectedDeliveryDate
	if in.CreatedBy != "" {
		cols["created_by"] = in.CreatedBy
	}
	if len(cols) > 0 {
		if err := s.repo.UpdateColumns(id, cols); err != nil {
			return nil, err
		}
	}
	return s.repo.GetWithDetails(id)
}

// UpdateFull replaces both the header and the detail lines of an existing PO,
// recomputing line totals and the grand total. It is the "edit existing PO"
// counterpart to Create.
func (s *PurchaseOrderService) UpdateFull(id uint, in CreatePOInput) (*model.PurchaseOrder, error) {
	po, err := s.repo.GetWithDetails(id)
	if po == nil || err != nil {
		return nil, errNotFound(id)
	}
	if in.SupplierID == 0 || len(in.Details) == 0 {
		return nil, errorsBadRequest("supplier_id and at least one detail are required")
	}
	po.SupplierID = in.SupplierID
	if in.OrderDate.IsZero() {
		po.OrderDate = time.Now()
	} else {
		po.OrderDate = in.OrderDate
	}
	po.ExpectedDelivery = in.ExpectedDeliveryDate
	if in.CreatedBy != "" {
		po.CreatedBy = in.CreatedBy
	}
	po.Details = nil
	po.TotalAmount = decimal.Zero
	for _, d := range in.Details {
		if d.OrderQty.LessThanOrEqual(decimal.Zero) {
			return nil, errorsBadRequest("order_qty must be positive")
		}
		lineTotal := d.OrderQty.Mul(d.UnitPrice)
		po.Details = append(po.Details, model.PurchaseOrderDetail{
			POID:        po.ID,
			MaterialID:  d.MaterialID,
			OrderQty:    d.OrderQty,
			UnitPrice:   d.UnitPrice,
			ReceivedQty: decimal.Zero,
			TotalPrice:  lineTotal,
		})
		po.TotalAmount = po.TotalAmount.Add(lineTotal)
	}
	if err := s.repo.UpdateWithDetails(po); err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(id)
}

// Get loads a PO with details.
func (s *PurchaseOrderService) Get(id uint) (*model.PurchaseOrder, error) {
	return s.repo.GetWithDetails(id)
}

// Delete removes a PO.
func (s *PurchaseOrderService) Delete(id uint) error {
	return s.repo.Delete(id)
}

// List returns a paginated PO list.
func (s *PurchaseOrderService) List(in PageInput) ([]model.PurchaseOrder, int64, error) {
	var (
		out   []model.PurchaseOrder
		total int64
	)
	if err := s.repo.List(repository.ListFilter{Page: in.Page, Keyword: in.Keyword}, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// SetStatus transitions a PO to a new status.
func (s *PurchaseOrderService) SetStatus(id uint, status string) error {
	po, err := s.repo.Get(id)
	if po == nil {
		return errNotFound(id)
	}
	if err != nil {
		return err
	}
	po.Status = status
	return s.repo.Update(po)
}
