package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"scm/internal/model"
	"scm/pkg/response"
	"scm/internal/service"
)

// RNDHandler exposes R&D products and bill-of-materials endpoints.
type RNDHandler struct {
	Products *service.ProductService
	BOMs     *service.BOMService
}

func NewRNDHandler(p *service.ProductService, b *service.BOMService) *RNDHandler {
	return &RNDHandler{Products: p, BOMs: b}
}

// ---- Products ----

func (h *RNDHandler) ProductList(c *gin.Context) {
	list, total, err := h.Products.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *RNDHandler) ProductGet(c *gin.Context) {
	m, err := h.Products.Get(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *RNDHandler) ProductCreate(c *gin.Context) {
	var m model.Product
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.Products.Create(tenantOf(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *RNDHandler) ProductUpdate(c *gin.Context) {
	var m model.Product
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.Products.Update(tenantOf(c), idParam(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *RNDHandler) ProductDelete(c *gin.Context) {
	if mapErr(c, h.Products.Delete(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// ---- BOMs ----

type bomRequest struct {
	BOMNo     string `json:"bom_no"`
	ProductID uint   `json:"product_id"`
	UnitQty   string `json:"unit_qty"`
	Remark    string `json:"remark"`
	Details   []struct {
		ComponentID uint   `json:"component_id"`
		QtyPerUnit  string `json:"qty_per_unit"`
		ScrapRate   string `json:"scrap_rate"`
		Remark      string `json:"remark"`
	} `json:"details"`
}

func (h *RNDHandler) toInput(req *bomRequest) service.BOMInput {
	in := service.BOMInput{BOMNo: req.BOMNo, ProductID: req.ProductID, Remark: req.Remark}
	if req.UnitQty != "" {
		in.UnitQty, _ = decimal.NewFromString(req.UnitQty)
	}
	for _, d := range req.Details {
		q, _ := decimal.NewFromString(d.QtyPerUnit)
		s, _ := decimal.NewFromString(d.ScrapRate)
		in.Details = append(in.Details, service.BOMDetailInput{
			ComponentID: d.ComponentID, QtyPerUnit: q, ScrapRate: s, Remark: d.Remark,
		})
	}
	return in
}

func (h *RNDHandler) BOMList(c *gin.Context) {
	list, total, err := h.BOMs.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *RNDHandler) BOMListByProduct(c *gin.Context) {
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	list, err := h.BOMs.ListByProduct(tenantOf(c), uint(pid))
	if mapErr(c, err) {
		return
	}
	response.OK(c, list)
}

func (h *RNDHandler) BOMGet(c *gin.Context) {
	b, err := h.BOMs.Get(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, b)
}

func (h *RNDHandler) BOMCreate(c *gin.Context) {
	var req bomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	b, err := h.BOMs.Create(tenantOf(c), h.toInput(&req))
	if mapErr(c, err) {
		return
	}
	response.OK(c, b)
}

func (h *RNDHandler) BOMUpdate(c *gin.Context) {
	var req bomRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	b, err := h.BOMs.Update(tenantOf(c), idParam(c), h.toInput(&req))
	if mapErr(c, err) {
		return
	}
	response.OK(c, b)
}

func (h *RNDHandler) BOMRelease(c *gin.Context) {
	b, err := h.BOMs.Release(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, b)
}

func (h *RNDHandler) BOMDelete(c *gin.Context) {
	if mapErr(c, h.BOMs.Delete(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}
