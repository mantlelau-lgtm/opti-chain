package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"scm/internal/pkg/response"
	"scm/internal/service"
)

// PurchaseOrderHandler exposes purchase-order endpoints.
type PurchaseOrderHandler struct{ svc *service.PurchaseOrderService }

func NewPurchaseOrderHandler(svc *service.PurchaseOrderService) *PurchaseOrderHandler {
	return &PurchaseOrderHandler{svc: svc}
}

// poCreateRequest is the JSON body for creating a PO.
type poCreateRequest struct {
	PONumber         string `json:"po_number"`
	SupplierID       uint   `json:"supplier_id"`
	OrderDate        string `json:"order_date"`
	ExpectedDelivery string `json:"expected_delivery"`
	Details          []struct {
		MaterialID uint   `json:"material_id"`
		OrderQty   string `json:"order_qty"`
		UnitPrice  string `json:"unit_price"`
		LocationID uint   `json:"location_id"`
	} `json:"details"`
}

func (h *PurchaseOrderHandler) List(c *gin.Context) {
	list, total, err := h.svc.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *PurchaseOrderHandler) Get(c *gin.Context) {
	po, err := h.svc.Get(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, po)
}

func (h *PurchaseOrderHandler) Create(c *gin.Context) {
	var req poCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	in := service.CreatePOInput{
		PONumber:   req.PONumber,
		SupplierID: req.SupplierID,
		CreatedBy:  actorUsername(c),
	}
	if req.OrderDate != "" {
		in.OrderDate = parseTime(req.OrderDate)
	} else {
		in.OrderDate = time.Now()
	}
	if req.ExpectedDelivery != "" {
		ed := parseTime(req.ExpectedDelivery)
		in.ExpectedDeliveryDate = &ed
	}
	for _, d := range req.Details {
		oq, _ := decimal.NewFromString(d.OrderQty)
		up, _ := decimal.NewFromString(d.UnitPrice)
		in.Details = append(in.Details, service.PODetailInput{
			MaterialID: d.MaterialID,
			OrderQty:   oq,
			UnitPrice:  up,
			LocationID: d.LocationID,
		})
	}
	po, err := h.svc.Create(tenantOf(c), in)
	if mapErr(c, err) {
		return
	}
	response.OK(c, po)
}

// Update edits a PO. A payload WITHOUT detail lines is a header-only edit
// (the SPA's edit form); with detail lines it is a full replace.
func (h *PurchaseOrderHandler) Update(c *gin.Context) {
	var req poCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	in := service.CreatePOInput{
		PONumber:   req.PONumber,
		SupplierID: req.SupplierID,
		CreatedBy:  actorUsername(c),
	}
	if req.OrderDate != "" {
		in.OrderDate = parseTime(req.OrderDate)
	}
	if req.ExpectedDelivery != "" {
		ed := parseTime(req.ExpectedDelivery)
		in.ExpectedDeliveryDate = &ed
	}
	if len(req.Details) == 0 {
		po, err := h.svc.UpdateHeader(tenantOf(c), idParam(c), in)
		if mapErr(c, err) {
			return
		}
		response.OK(c, po)
		return
	}
	for _, d := range req.Details {
		oq, _ := decimal.NewFromString(d.OrderQty)
		up, _ := decimal.NewFromString(d.UnitPrice)
		in.Details = append(in.Details, service.PODetailInput{
			MaterialID: d.MaterialID,
			OrderQty:   oq,
			UnitPrice:  up,
			LocationID: d.LocationID,
		})
	}
	po, err := h.svc.UpdateFull(tenantOf(c), idParam(c), in)
	if mapErr(c, err) {
		return
	}
	response.OK(c, po)
}

// SetStatus transitions a PO status (e.g. DRAFT -> APPROVED).
func (h *PurchaseOrderHandler) SetStatus(c *gin.Context) {
	var body struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.svc.SetStatus(tenantOf(c), idParam(c), body.Status); mapErr(c, err) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c), "status": body.Status})
}

func (h *PurchaseOrderHandler) Delete(c *gin.Context) {
	if mapErr(c, h.svc.Delete(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

func parseTime(s string) time.Time {
	// Accept RFC3339 or "2006-01-02" date.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return time.Now()
}
