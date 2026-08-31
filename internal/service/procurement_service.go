package service

import (
	"time"

	"github.com/shopspring/decimal"

	"scm/internal/model"
	"scm/internal/repository"
)

// PurchaseOrderService owns purchase-order lifecycle logic. Every method
// takes the tenant id first (threaded from the authenticated Actor).
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
// supplier must exist (same tenant) and be approved (SOP 准入管控).
func (s *PurchaseOrderService) Create(t uint, in CreatePOInput) (*model.PurchaseOrder, error) {
	if in.SupplierID == 0 || len(in.Details) == 0 {
		return nil, errorsBadRequest("supplier_id and at least one detail are required")
	}
	supplier, err := s.suppliers.Get(t, in.SupplierID)
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
		TotalAmount:      decimal.Zero,
	}
	po.CreatedBy = in.CreatedBy
	po.UpdatedBy = in.CreatedBy
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
	if err := s.repo.CreateWithDetails(t, po); err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(t, po.ID)
}

// UpdateHeader edits only the header fields supplied by the caller; details,
// totals, status and received progress are never touched (so editing a
// partially received PO cannot wipe its fulfillment state).
func (s *PurchaseOrderService) UpdateHeader(t, id uint, in CreatePOInput) (*model.PurchaseOrder, error) {
	po, err := s.repo.Get(t, id)
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
		supplier, err := s.suppliers.Get(t, in.SupplierID)
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
		cols["updated_by"] = in.CreatedBy
	}
	if len(cols) > 0 {
		if err := s.repo.UpdateColumns(t, id, cols); err != nil {
			return nil, err
		}
	}
	return s.repo.GetWithDetails(t, id)
}

// UpdateFull replaces both the header and the detail lines of an existing PO,
// recomputing line totals and the grand total. It is the "edit existing PO"
// counterpart to Create.
func (s *PurchaseOrderService) UpdateFull(t, id uint, in CreatePOInput) (*model.PurchaseOrder, error) {
	po, err := s.repo.GetWithDetails(t, id)
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
		po.UpdatedBy = in.CreatedBy
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
	if err := s.repo.UpdateWithDetails(t, po); err != nil {
		return nil, err
	}
	return s.repo.GetWithDetails(t, id)
}

// Get loads a PO with details.
func (s *PurchaseOrderService) Get(t, id uint) (*model.PurchaseOrder, error) {
	return s.repo.GetWithDetails(t, id)
}

// Delete removes a PO.
func (s *PurchaseOrderService) Delete(t, id uint) error {
	return s.repo.Delete(t, id)
}

// List returns a paginated PO list within the tenant.
func (s *PurchaseOrderService) List(t uint, in PageInput) ([]model.PurchaseOrder, int64, error) {
	var (
		out   []model.PurchaseOrder
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.repo.List(f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// SetStatus transitions a PO to a new status.
func (s *PurchaseOrderService) SetStatus(t, id uint, status string) error {
	po, err := s.repo.Get(t, id)
	if po == nil {
		return errNotFound(id)
	}
	if err != nil {
		return err
	}
	po.Status = status
	return s.repo.Update(t, po)
}
