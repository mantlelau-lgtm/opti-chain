package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"scm/internal/pkg/response"
	"scm/internal/service"
)

// BOMOrderHandler exposes the "order by BOM" flow: preview the supplier split,
// then confirm to create one PO per supplier.
type BOMOrderHandler struct{ svc *service.BOMOrderService }

func NewBOMOrderHandler(s *service.BOMOrderService) *BOMOrderHandler {
	return &BOMOrderHandler{svc: s}
}

type bomOrderRequest struct {
	ProductID uint   `json:"product_id"`
	Qty       string `json:"qty"`
	OrderDate string `json:"order_date"`
}

func (h *BOMOrderHandler) Preview(c *gin.Context) {
	var req bomOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	q, _ := decimal.NewFromString(req.Qty)
	plan, err := h.svc.Preview(tenantOf(c), req.ProductID, q)
	if mapErr(c, err) {
		return
	}
	response.OK(c, plan)
}

func (h *BOMOrderHandler) Confirm(c *gin.Context) {
	var req bomOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	q, _ := decimal.NewFromString(req.Qty)
	var od = parseTime(req.OrderDate)
	orders, err := h.svc.Create(tenantOf(c), req.ProductID, q, od)
	if mapErr(c, err) {
		return
	}
	response.OK(c, orders)
}
