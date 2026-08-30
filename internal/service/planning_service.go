package service

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"scm/internal/model"
	"scm/internal/pkg/query"
	"scm/internal/repository"
)

// PlanningService orchestrates demand handling and MRP computation.
//
// It depends on the PurchaseOrderService (not the raw repo) so that converting
// an MRP result into a PO reuses the PO module's own validation, line-total and
// transaction logic, keeping cross-module behaviour consistent.
type PlanningService struct {
	demand   *repository.DemandRepo
	mrp      *repository.MrpResultRepo
	stock    *repository.StockRepo
	posvc    *PurchaseOrderService
	supplier *repository.SupplierRepo
	db       *gorm.DB
}

// PlanningDeps groups the dependencies for PlanningService.
type PlanningDeps struct {
	Demand   *repository.DemandRepo
	Mrp      *repository.MrpResultRepo
	Stock    *repository.StockRepo
	POSvc    *PurchaseOrderService
	Supplier *repository.SupplierRepo
	DB       *gorm.DB
}

func NewPlanningService(d PlanningDeps) *PlanningService {
	return &PlanningService{
		demand:   d.Demand,
		mrp:      d.Mrp,
		stock:    d.Stock,
		posvc:    d.POSvc,
		supplier: d.Supplier,
		db:       d.DB,
	}
}

// ---- Demand CRUD ----

// Create persists a demand and returns it.
func (s *PlanningService) CreateDemand(d *model.Demand) error {
	if d.MaterialID == 0 || d.DemandQty.LessThanOrEqual(decimal.Zero) {
		return errorsBadRequest("material_id and positive demand_qty are required")
	}
	return s.demand.Create(d)
}

func (s *PlanningService) UpdateDemand(id uint, d *model.Demand) error {
	d.ID = id
	return s.demand.Update(d)
}

func (s *PlanningService) GetDemand(id uint) (*model.Demand, error) {
	return s.demand.Get(id)
}

func (s *PlanningService) DeleteDemand(id uint) error {
	return s.demand.Delete(id)
}

func (s *PlanningService) ListDemands(in PageInput) ([]model.Demand, int64, error) {
	var (
		out   []model.Demand
		total int64
	)
	if err := s.demand.List(repository.ListFilter{Page: in.Page, Keyword: in.Keyword}, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// ---- MRP ----

// ComputeMRP runs a lightweight material-requirements calculation.
//
// For each open demand it aggregates gross demand by material, then computes:
//
//	suggested_po_qty = gross_demand + safety(min_stock) - current_stock - on_order
//
// A suggested purchase order is produced only when the result is positive. The
// results are persisted as a new MRP batch.
func (s *PlanningService) ComputeMRP(batchNo string) ([]model.MrpResult, error) {
	// 1) aggregate open demand per material.
	byMat, err := s.demand.SumOpenByMaterial()
	if err != nil {
		return nil, err
	}
	if len(byMat) == 0 {
		return nil, nil
	}

	// load safety-stock (min_stock) for the involved materials.
	ids := make([]uint, 0, len(byMat))
	for matID := range byMat {
		ids = append(ids, matID)
	}
	var mats []model.Material
	if err := s.db.Where("id IN ?", ids).Find(&mats).Error; err != nil {
		return nil, err
	}
	minStock := make(map[uint]decimal.Decimal, len(mats))
	for _, m := range mats {
		minStock[m.ID] = m.MinStock
	}

	var results []model.MrpResult
	for matID, gross := range byMat {
		grossD := decimal.NewFromFloat(gross)

		// current on-hand across all warehouses/locations.
		current, err := s.stock.SumByMaterial(matID)
		if err != nil {
			return nil, err
		}

		// on-order = open purchase order qty not yet received.
		onOrder, err := s.sumOnOrder(matID)
		if err != nil {
			return nil, err
		}

		// suggested = gross + safety - current - on_order, floored at 0.
		net := grossD.Add(minStock[matID]).Sub(current).Sub(onOrder)
		suggested := decimal.Zero
		if net.IsPositive() {
			suggested = net
		}

		results = append(results, model.MrpResult{
			MrpNumber:       batchNo,
			MaterialID:      matID,
			CurrentStock:    current,
			OnOrderQty:      onOrder,
			GrossDemand:     grossD,
			SuggestedPOQty:  suggested,
			SuggestedPODate: ptrTime(time.Now()),
			Status:          model.MrpStatusPending,
		})
	}

	if err := s.mrp.BatchCreate(results); err != nil {
		return nil, err
	}
	// mark processed demands as generated.
	_ = s.db.Model(&model.Demand{}).Where("status = ?", model.DemandStatusOpen).
		Update("status", model.DemandStatusGenerated).Error

	return results, nil
}

// sumOnOrder returns the not-yet-received qty of open POs for a material:
//
//	sum(order_qty - received_qty) for active (non-cancelled/completed) POs.
func (s *PlanningService) sumOnOrder(matID uint) (decimal.Decimal, error) {
	var res struct {
		Qty decimal.Decimal
	}
	err := s.db.Raw(
		"SELECT COALESCE(SUM(d.order_qty - d.received_qty), 0) AS qty FROM pur_order_detail d "+
			"JOIN pur_order p ON p.id = d.po_id "+
			"WHERE d.material_id = ? AND p.status NOT IN ('CANCELLED','COMPLETED')",
		matID,
	).Scan(&res).Error
	if err != nil {
		return decimal.Zero, err
	}
	return res.Qty, nil
}

// ListMrp returns paginated MRP results.
func (s *PlanningService) ListMrp(in PageInput) ([]model.MrpResult, int64, error) {
	var (
		out   []model.MrpResult
		total int64
	)
	if err := s.mrp.List(repository.ListFilter{Page: in.Page, Keyword: in.Keyword}, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *PlanningService) GetMrp(id uint) (*model.MrpResult, error) {
	return s.mrp.Get(id)
}

func (s *PlanningService) DeleteMrp(id uint) error {
	return s.mrp.Delete(id)
}

// ConvertMRP turns a single MRP result into a purchase order.
//
// The suggested purchase quantity becomes a one-line PO for the material. The
// PO number defaults to "MRP-<mrpNumber>-<materialID>" and can be overridden.
// The MRP result is then flipped to CONVERTED to record that the suggestion has
// been actioned, so it is not converted twice.
func (s *PlanningService) ConvertMRP(mrpID uint, poNumber string) (*model.PurchaseOrder, error) {
	mrp, err := s.mrp.Get(mrpID)
	if mrp == nil || err != nil {
		return nil, errNotFound(mrpID)
	}
	if !mrp.SuggestedPOQty.IsPositive() {
		return nil, errorsBadRequest("no positive suggested quantity to convert")
	}
	if mrp.Status == model.MrpStatusConverted {
		return nil, errorsBadRequest("mrp result already converted")
	}

	// A PO must reference a supplier; pick any enabled one, preferring the first.
	var supplierID uint
	var suppliers []model.Supplier
	_ = s.supplier.List(repository.ListFilter{Page: query.Page{Page: 1, Size: 1}}, &suppliers, nil)
	if len(suppliers) > 0 {
		supplierID = suppliers[0].ID
	}
	if supplierID == 0 {
		return nil, errorsBadRequest("no supplier available to create a purchase order")
	}

	number := poNumber
	if number == "" {
		number = "MRP-" + mrp.MrpNumber + "-" + itob(mrp.MaterialID)
	}

	po, err := s.posvc.Create(CreatePOInput{
		PONumber:   number,
		SupplierID: supplierID,
		OrderDate:  time.Now(),
		Details:    []PODetailInput{{MaterialID: mrp.MaterialID, OrderQty: mrp.SuggestedPOQty, UnitPrice: decimal.Zero}},
	})
	if err != nil {
		return nil, err
	}

	// Mark the suggestion as actioned.
	mrp.Status = model.MrpStatusConverted
	if err := s.mrp.Update(mrp); err != nil {
		return nil, err
	}
	return po, nil
}

func ptrTime(t time.Time) *time.Time { return &t }
