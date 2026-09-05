package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"scm/pkg/response"
	"scm/internal/service"
)

// ReceivingHandler exposes purchase-receiving endpoints.
type ReceivingHandler struct{ svc *service.ReceivingService }

func NewReceivingHandler(svc *service.ReceivingService) *ReceivingHandler {
	return &ReceivingHandler{svc: svc}
}

// receiveRequest is the JSON body for one receiving round.
type receiveRequest struct {
	ReceiptNumber string `json:"receipt_number"`
	WarehouseID   uint   `json:"warehouse_id"`
	ReceiptDate   string `json:"receipt_date"`
	Remark        string `json:"remark"`
	Details       []struct {
		PODetailID   uint   `json:"po_detail_id"`
		LocationID   uint   `json:"location_id"`
		PassedQty    string `json:"passed_qty"`
		RejectedQty  string `json:"rejected_qty"`
		RejectReason string `json:"reject_reason"`
	} `json:"details"`
}

// Receive books one receiving round against a PO: accepted qty goes into
// stock, rejected qty is recorded for supplier return without touching stock.
func (h *ReceivingHandler) Receive(c *gin.Context) {
	var req receiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	in := service.ReceiveInput{
		ReceiptNumber: req.ReceiptNumber,
		WarehouseID:   req.WarehouseID,
		Remark:        req.Remark,
	}
	if req.ReceiptDate != "" {
		in.ReceiptDate = parseTime(req.ReceiptDate)
	}
	for _, d := range req.Details {
		passed, _ := decimal.NewFromString(d.PassedQty)
		rejected, _ := decimal.NewFromString(d.RejectedQty)
		in.Details = append(in.Details, service.ReceiveDetailInput{
			PODetailID:   d.PODetailID,
			LocationID:   d.LocationID,
			PassedQty:    passed,
			RejectedQty:  rejected,
			RejectReason: d.RejectReason,
		})
	}
	rc, err := h.svc.Receive(tenantOf(c), idParam(c), in)
	if mapErr(c, err) {
		return
	}
	response.OK(c, rc)
}

// Receipts lists the receiving rounds of a PO.
func (h *ReceivingHandler) Receipts(c *gin.Context) {
	list, err := h.svc.ListReceipts(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, list)
}
