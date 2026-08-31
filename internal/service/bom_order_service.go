package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"scm/internal/model"
	"scm/internal/repository"
)

// BOMOrderService turns a product's BOM into purchase orders: it expands the
// default BOM, auto-selects a supplier per material (preferred first, then
// lowest price) and splits the result into one PO per supplier.
type BOMOrderService struct {
	bom       *repository.BOMRepo
	supMat    *repository.SupplierMaterialRepo
	suppliers *repository.SupplierRepo
	materials *repository.MaterialRepo
	posvc     *PurchaseOrderService
	db        *gorm.DB
}

// BOMOrderDeps groups the dependencies a BOMOrderService needs.
type BOMOrderDeps struct {
	BOM       *repository.BOMRepo
	SupMat    *repository.SupplierMaterialRepo
	Suppliers *repository.SupplierRepo
	Materials *repository.MaterialRepo
	POSvc     *PurchaseOrderService
	DB        *gorm.DB
}

func NewBOMOrderService(d BOMOrderDeps) *BOMOrderService {
	return &BOMOrderService{
		bom: d.BOM, supMat: d.SupMat, suppliers: d.Suppliers,
		materials: d.Materials, posvc: d.POSvc, db: d.DB,
	}
}

// BOMOrderItem is one expanded line with its resolved supplier price.
type BOMOrderItem struct {
	MaterialID   uint            `json:"material_id"`
	MaterialName string          `json:"material_name"`
	Qty          decimal.Decimal `json:"qty"`
	UnitPrice    decimal.Decimal `json:"unit_price"`
}

// BOMOrderGroup groups expanded lines by supplier (one PO per group).
type BOMOrderGroup struct {
	SupplierID   uint           `json:"supplier_id"`
	SupplierName string         `json:"supplier_name"`
	Items        []BOMOrderItem `json:"items"`
}

// BOMOrderPlan is the split-plan preview.
type BOMOrderPlan struct {
	Groups   []BOMOrderGroup `json:"groups"`
	Warnings []string        `json:"warnings"`
}

// BOMOrderLine is one product and its required quantity in a multi-BOM order.
type BOMOrderLine struct {
	ProductID uint            `json:"product_id"`
	Qty       decimal.Decimal `json:"qty"`
}

// Preview expands the products' default BOMs (aggregating shared materials),
// resolves a supplier per material and groups the result by supplier. No writes.
func (s *BOMOrderService) Preview(t uint, items []BOMOrderLine) (*BOMOrderPlan, error) {
	return s.resolve(t, items)
}

// Create re-resolves the plan and creates one DRAFT PO per supplier.
func (s *BOMOrderService) Create(t uint, items []BOMOrderLine, orderDate time.Time) ([]*model.PurchaseOrder, error) {
	plan, err := s.resolve(t, items)
	if err != nil {
		return nil, err
	}
	if len(plan.Warnings) > 0 {
		return nil, errorsBadRequest("存在无可用供应商的物料，无法下单：" + strings.Join(plan.Warnings, "；"))
	}
	if orderDate.IsZero() {
		orderDate = time.Now()
	}
	ts := orderDate.Format("20060102150405")
	var orders []*model.PurchaseOrder
	for i, g := range plan.Groups {
		details := make([]PODetailInput, 0, len(g.Items))
		for _, it := range g.Items {
			details = append(details, PODetailInput{
				MaterialID: it.MaterialID,
				OrderQty:   it.Qty,
				UnitPrice:  it.UnitPrice,
			})
		}
		po, err := s.posvc.Create(t, CreatePOInput{
			PONumber:   fmt.Sprintf("BOM-%s-%d", ts, i+1),
			SupplierID: g.SupplierID,
			OrderDate:  orderDate,
			Details:    details,
		})
		if err != nil {
			return nil, err
		}
		orders = append(orders, po)
	}
	return orders, nil
}

func (s *BOMOrderService) resolve(t uint, items []BOMOrderLine) (*BOMOrderPlan, error) {
	if len(items) == 0 {
		return nil, errorsBadRequest("at least one product is required")
	}
	// Aggregate material requirements across all products (shared materials sum).
	matQty := map[uint]decimal.Decimal{}
	for _, it := range items {
		if it.Qty.LessThanOrEqual(decimal.Zero) {
			return nil, errorsBadRequest("product qty must be positive")
		}
		bom, err := s.bom.DefaultByProduct(t, it.ProductID)
		if err != nil {
			return nil, err
		}
		if bom == nil {
			return nil, errorsBadRequest(fmt.Sprintf("product %d has no released BOM", it.ProductID))
		}
		for _, d := range bom.Details {
			cur := matQty[d.ComponentID]
			matQty[d.ComponentID] = cur.Add(d.QtyPerUnit.Mul(it.Qty))
		}
	}
	if len(matQty) == 0 {
		return nil, errorsBadRequest("selected BOMs have no components")
	}
	// Deterministic iteration order over materials.
	matIDs := make([]uint, 0, len(matQty))
	for id := range matQty {
		matIDs = append(matIDs, id)
	}
	sort.Slice(matIDs, func(i, j int) bool { return matIDs[i] < matIDs[j] })

	plan := &BOMOrderPlan{Groups: []BOMOrderGroup{}, Warnings: []string{}}
	groups := map[uint]*BOMOrderGroup{}
	var order []uint
	for _, matID := range matIDs {
		qty := matQty[matID]
		name := fmt.Sprint(matID)
		if mat, _ := s.materials.Get(t, matID); mat != nil {
			name = mat.Name
		}
		rel, err := s.pickSupplier(t, matID)
		if err != nil {
			return nil, err
		}
		if rel == nil {
			plan.Warnings = append(plan.Warnings, name+"（无可用供应商）")
			continue
		}
		sname := fmt.Sprint(rel.SupplierID)
		if supplier, _ := s.suppliers.Get(t, rel.SupplierID); supplier != nil {
			sname = supplier.Name
		}
		g, ok := groups[rel.SupplierID]
		if !ok {
			g = &BOMOrderGroup{SupplierID: rel.SupplierID, SupplierName: sname}
			groups[rel.SupplierID] = g
			order = append(order, rel.SupplierID)
		}
		g.Items = append(g.Items, BOMOrderItem{
			MaterialID:   matID,
			MaterialName: name,
			Qty:          qty,
			UnitPrice:    rel.UnitPrice,
		})
	}
	for _, sid := range order {
		plan.Groups = append(plan.Groups, *groups[sid])
	}
	return plan, nil
}

// pickSupplier returns the preferred (is_preferred) or cheapest APPROVED
// supplier-material relationship for a material; nil when none qualifies.
func (s *BOMOrderService) pickSupplier(t, materialID uint) (*model.SupplierMaterial, error) {
	var rels []model.SupplierMaterial
	if err := s.supMat.List(t, 0, materialID, &rels); err != nil {
		return nil, err
	}
	var best *model.SupplierMaterial
	for i := range rels {
		r := &rels[i]
		if r.Status != 1 {
			continue
		}
		supplier, err := s.suppliers.Get(t, r.SupplierID)
		if err != nil {
			return nil, err
		}
		if supplier == nil || supplier.Status != 1 || supplier.AuditStatus != model.AuditApproved {
			continue
		}
		if r.IsPreferred {
			return r, nil
		}
		if best == nil || r.UnitPrice.LessThan(best.UnitPrice) {
			best = r
		}
	}
	return best, nil
}
