package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"scm/pkg/response"
	"scm/internal/service"
)

// BOMOrderHandler exposes the "order by BOM" flow: preview the supplier split,
// then confirm to create one PO per supplier.
type BOMOrderHandler struct{ svc *service.BOMOrderService }

func NewBOMOrderHandler(s *service.BOMOrderService) *BOMOrderHandler {
	return &BOMOrderHandler{svc: s}
}

type bomOrderRequest struct {
	Items     []bomOrderItemReq `json:"items"`
	OrderDate string            `json:"order_date"`
}

type bomOrderItemReq struct {
	ProductID uint   `json:"product_id"`
	Qty       string `json:"qty"`
}

func (h *BOMOrderHandler) toLines(req *bomOrderRequest) []service.BOMOrderLine {
	lines := make([]service.BOMOrderLine, 0, len(req.Items))
	for _, it := range req.Items {
		q, _ := decimal.NewFromString(it.Qty)
		lines = append(lines, service.BOMOrderLine{ProductID: it.ProductID, Qty: q})
	}
	return lines
}

func (h *BOMOrderHandler) Preview(c *gin.Context) {
	var req bomOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	plan, err := h.svc.Preview(tenantOf(c), h.toLines(&req))
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
	od := parseTime(req.OrderDate)
	orders, err := h.svc.Create(tenantOf(c), h.toLines(&req), od)
	if mapErr(c, err) {
		return
	}
	response.OK(c, orders)
}
