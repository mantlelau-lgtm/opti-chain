package handler

import (
	"github.com/gin-gonic/gin"

	"scm/internal/model"
	"scm/internal/service"
	"scm/pkg/response"
)

// BaseDataHandler exposes the base-data CRUD endpoints.
type BaseDataHandler struct {
	Material  *service.MaterialService
	Supplier  *service.SupplierService
	Warehouse *service.WarehouseService
}

func NewBaseDataHandler(m *service.MaterialService, su *service.SupplierService,
	w *service.WarehouseService) *BaseDataHandler {
	return &BaseDataHandler{Material: m, Supplier: su, Warehouse: w}
}

// ---- Material ----

func (h *BaseDataHandler) MaterialList(c *gin.Context) {
	list, total, err := h.Material.List(c.Request.Context(), tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *BaseDataHandler) MaterialGet(c *gin.Context) {
	m, err := h.Material.Get(c.Request.Context(), tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) MaterialCreate(c *gin.Context) {
	var m model.Material
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	m.CreatedBy = actorUsername(c)
	m.UpdatedBy = actorUsername(c)
	if err := h.Material.Create(c.Request.Context(), tenantOf(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) MaterialUpdate(c *gin.Context) {
	var m model.Material
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	m.UpdatedBy = actorUsername(c)
	if err := h.Material.Update(c.Request.Context(), tenantOf(c), idParam(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) MaterialDelete(c *gin.Context) {
	if mapErr(c, h.Material.Delete(c.Request.Context(), tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// ---- Supplier ----

func (h *BaseDataHandler) SupplierList(c *gin.Context) {
	list, total, err := h.Supplier.List(c.Request.Context(), tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *BaseDataHandler) SupplierGet(c *gin.Context) {
	m, err := h.Supplier.Get(c.Request.Context(), tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) SupplierCreate(c *gin.Context) {
	var m model.Supplier
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	m.CreatedBy = actorUsername(c)
	m.UpdatedBy = actorUsername(c)
	if err := h.Supplier.Create(c.Request.Context(), tenantOf(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) SupplierUpdate(c *gin.Context) {
	var m model.Supplier
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	m.UpdatedBy = actorUsername(c)
	if err := h.Supplier.Update(c.Request.Context(), tenantOf(c), idParam(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) SupplierDelete(c *gin.Context) {
	if mapErr(c, h.Supplier.Delete(c.Request.Context(), tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// SupplierSetAudit transitions the supplier qualification state (准入管控).
func (h *BaseDataHandler) SupplierSetAudit(c *gin.Context) {
	var body struct {
		AuditStatus string `json:"audit_status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	m, err := h.Supplier.SetAuditStatus(c.Request.Context(), tenantOf(c), idParam(c), body.AuditStatus)
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

// ---- Warehouse ----

func (h *BaseDataHandler) WarehouseList(c *gin.Context) {
	list, total, err := h.Warehouse.List(c.Request.Context(), tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *BaseDataHandler) WarehouseGet(c *gin.Context) {
	m, err := h.Warehouse.Get(c.Request.Context(), tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) WarehouseCreate(c *gin.Context) {
	var m model.Warehouse
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	m.CreatedBy = actorUsername(c)
	m.UpdatedBy = actorUsername(c)
	if err := h.Warehouse.Create(c.Request.Context(), tenantOf(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) WarehouseUpdate(c *gin.Context) {
	var m model.Warehouse
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	m.UpdatedBy = actorUsername(c)
	if err := h.Warehouse.Update(c.Request.Context(), tenantOf(c), idParam(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) WarehouseDelete(c *gin.Context) {
	if mapErr(c, h.Warehouse.Delete(c.Request.Context(), tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}
