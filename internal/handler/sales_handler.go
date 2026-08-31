package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"scm/internal/model"
	"scm/internal/pkg/response"
	"scm/internal/service"
)

// SalesHandler exposes customer master-data and sales-order endpoints.
type SalesHandler struct {
	Customers *service.CustomerService
	Orders    *service.SalesOrderService
}

func NewSalesHandler(c *service.CustomerService, o *service.SalesOrderService) *SalesHandler {
	return &SalesHandler{Customers: c, Orders: o}
}

// ---- Customer ----

func (h *SalesHandler) CustomerList(c *gin.Context) {
	list, total, err := h.Customers.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *SalesHandler) CustomerGet(c *gin.Context) {
	m, err := h.Customers.Get(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *SalesHandler) CustomerCreate(c *gin.Context) {
	var m model.Customer
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.Customers.Create(tenantOf(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *SalesHandler) CustomerUpdate(c *gin.Context) {
	var m model.Customer
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.Customers.Update(tenantOf(c), idParam(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *SalesHandler) CustomerDelete(c *gin.Context) {
	if mapErr(c, h.Customers.Delete(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// ---- Sale order ----

// soCreateRequest is the JSON body for creating a sales order.
type soCreateRequest struct {
	SONumber   string `json:"so_number"`
	CustomerID uint   `json:"customer_id"`
	OrderDate  string `json:"order_date"`
	CreatedBy  string `json:"created_by"`
	Details    []struct {
		MaterialID uint   `json:"material_id"`
		Qty        string `json:"qty"`
		UnitPrice  string `json:"unit_price"`
	} `json:"details"`
}

func (h *SalesHandler) List(c *gin.Context) {
	list, total, err := h.Orders.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *SalesHandler) Get(c *gin.Context) {
	so, err := h.Orders.Get(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, so)
}

func (h *SalesHandler) Create(c *gin.Context) {
	var req soCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	in := service.CreateSOInput{
		SONumber:   req.SONumber,
		CustomerID: req.CustomerID,
		CreatedBy:  req.CreatedBy,
	}
	if req.OrderDate != "" {
		in.OrderDate = parseTime(req.OrderDate)
	} else {
		in.OrderDate = time.Now()
	}
	for _, d := range req.Details {
		qty, _ := decimal.NewFromString(d.Qty)
		up, _ := decimal.NewFromString(d.UnitPrice)
		in.Details = append(in.Details, service.SODetailInput{
			MaterialID: d.MaterialID,
			Qty:        qty,
			UnitPrice:  up,
		})
	}
	so, err := h.Orders.Create(tenantOf(c), in)
	if mapErr(c, err) {
		return
	}
	response.OK(c, so)
}

func (h *SalesHandler) Delete(c *gin.Context) {
	if mapErr(c, h.Orders.Delete(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// Approve locks the available stock for every line and consumes credit.
func (h *SalesHandler) Approve(c *gin.Context) {
	so, err := h.Orders.Approve(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, so)
}

// Cancel releases locks + credit held by an approved order.
func (h *SalesHandler) Cancel(c *gin.Context) {
	so, err := h.Orders.Cancel(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, so)
}
