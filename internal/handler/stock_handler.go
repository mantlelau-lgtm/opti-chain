package handler

import (
	"github.com/gin-gonic/gin"

	"scm/pkg/response"
	"scm/internal/service"
)

// StockHandler exposes real-time stock endpoints.
type StockHandler struct{ svc *service.StockService }

func NewStockHandler(svc *service.StockService) *StockHandler {
	return &StockHandler{svc: svc}
}

func (h *StockHandler) List(c *gin.Context) {
	list, total, err := h.svc.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}
