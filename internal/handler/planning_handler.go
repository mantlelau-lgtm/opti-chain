package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"scm/internal/model"
	"scm/internal/pkg/response"
	"scm/internal/service"
)

// PlanningHandler exposes demand + MRP endpoints.
type PlanningHandler struct{ svc *service.PlanningService }

func NewPlanningHandler(svc *service.PlanningService) *PlanningHandler {
	return &PlanningHandler{svc: svc}
}

// ---- Demand ----

func (h *PlanningHandler) DemandList(c *gin.Context) {
	list, total, err := h.svc.ListDemands(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *PlanningHandler) DemandGet(c *gin.Context) {
	m, err := h.svc.GetDemand(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

// demandRequest is the JSON body for demand create/update. The date arrives as
// a string so both "YYYY-MM-DD" (the SPA) and RFC3339 are accepted via
// parseTime, mirroring the PO handler.
type demandRequest struct {
	DemandNumber string          `json:"demand_number"`
	MaterialID   uint            `json:"material_id"`
	DemandQty    decimal.Decimal `json:"demand_qty"`
	DemandDate   string          `json:"demand_date"`
	SourceType   string          `json:"source_type"`
}

func (h *PlanningHandler) DemandCreate(c *gin.Context) {
	var req demandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	d := &model.Demand{
		DemandNumber: req.DemandNumber,
		MaterialID:   req.MaterialID,
		DemandQty:    req.DemandQty,
		DemandDate:   parseTime(req.DemandDate),
		SourceType:   req.SourceType,
	}
	if err := h.svc.CreateDemand(tenantOf(c), d); mapErr(c, err) {
		return
	}
	response.OK(c, d)
}

func (h *PlanningHandler) DemandUpdate(c *gin.Context) {
	var req demandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	d := &model.Demand{
		DemandNumber: req.DemandNumber,
		MaterialID:   req.MaterialID,
		DemandQty:    req.DemandQty,
		DemandDate:   parseTime(req.DemandDate),
		SourceType:   req.SourceType,
	}
	if err := h.svc.UpdateDemand(tenantOf(c), idParam(c), d); mapErr(c, err) {
		return
	}
	response.OK(c, d)
}

func (h *PlanningHandler) DemandDelete(c *gin.Context) {
	if mapErr(c, h.svc.DeleteDemand(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// ---- MRP ----

func (h *PlanningHandler) MrpList(c *gin.Context) {
	list, total, err := h.svc.ListMrp(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *PlanningHandler) MrpGet(c *gin.Context) {
	m, err := h.svc.GetMrp(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *PlanningHandler) MrpDelete(c *gin.Context) {
	if mapErr(c, h.svc.DeleteMrp(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// ComputeMRP triggers a material-requirements calculation batch.
func (h *PlanningHandler) ComputeMRP(c *gin.Context) {
	batch := "MRP" + time.Now().Format("20060102150405")
	results, err := h.svc.ComputeMRP(tenantOf(c), batch)
	if mapErr(c, err) {
		return
	}
	response.OK(c, gin.H{"mrp_number": batch, "results": results})
}

// MrpConvert turns a single MRP result into a purchase order.
func (h *PlanningHandler) MrpConvert(c *gin.Context) {
	var body struct {
		PONumber string `json:"po_number"`
	}
	// Body is optional; bind tolerates an empty payload.
	_ = c.ShouldBindJSON(&body)
	po, err := h.svc.ConvertMRP(tenantOf(c), idParam(c), body.PONumber)
	if mapErr(c, err) {
		return
	}
	response.OK(c, po)
}
