package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"scm/internal/pkg/response"
	"scm/internal/service"
)

// InventoryHandler exposes inventory-movement and stock endpoints.
type InventoryHandler struct{ svc *service.InventoryService }

func NewInventoryHandler(svc *service.InventoryService) *InventoryHandler {
	return &InventoryHandler{svc: svc}
}

// moveRequest is the JSON body for an in/out movement.
type moveRequest struct {
	OrderNumber    string `json:"order_number"`
	OrderType      string `json:"order_type"`
	RefOrderNumber string `json:"ref_order_number"`
	WarehouseID    uint   `json:"warehouse_id"`
	Details        []struct {
		MaterialID uint   `json:"material_id"`
		LocationID uint   `json:"location_id"`
		Qty        string `json:"qty"`
	} `json:"details"`
}

// toInput converts a request into a service MoveInput.
func toMove(in moveRequest) service.MoveInput {
	mi := service.MoveInput{
		OrderNumber:    in.OrderNumber,
		OrderType:      in.OrderType,
		RefOrderNumber: in.RefOrderNumber,
		WarehouseID:    in.WarehouseID,
	}
	for _, d := range in.Details {
		q, _ := decimal.NewFromString(d.Qty)
		mi.Details = append(mi.Details, service.MoveDetailInput{
			MaterialID: d.MaterialID,
			LocationID: d.LocationID,
			Qty:        q,
		})
	}
	return mi
}

func (h *InventoryHandler) MoveIn(c *gin.Context) {
	var req moveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	req.OrderType = "PURCHASE_IN"
	o, err := h.svc.MoveIn(toMove(req))
	if mapErr(c, err) {
		return
	}
	response.OK(c, o)
}

func (h *InventoryHandler) MoveOut(c *gin.Context) {
	var req moveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	req.OrderType = "SALE_OUT"
	o, err := h.svc.MoveOut(toMove(req))
	if mapErr(c, err) {
		return
	}
	response.OK(c, o)
}

func (h *InventoryHandler) ListOrders(c *gin.Context) {
	list, total, err := h.svc.ListOrders(parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *InventoryHandler) GetOrder(c *gin.Context) {
	o, err := h.svc.GetOrder(idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, o)
}

func (h *InventoryHandler) DeleteOrder(c *gin.Context) {
	if mapErr(c, h.svc.DeleteOrder(idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

func (h *InventoryHandler) ListLogs(c *gin.Context) {
	list, total, err := h.svc.ListLogs(parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}
