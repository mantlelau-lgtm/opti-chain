package handler

import (
	"github.com/gin-gonic/gin"

	"scm/internal/model"
	"scm/internal/pkg/response"
	"scm/internal/service"
)

// BaseDataHandler exposes the base-data CRUD endpoints.
type BaseDataHandler struct {
	Material  *service.MaterialService
	Supplier  *service.SupplierService
	Warehouse *service.WarehouseService
	Location  *service.LocationService
}

func NewBaseDataHandler(m *service.MaterialService, su *service.SupplierService,
	w *service.WarehouseService, l *service.LocationService) *BaseDataHandler {
	return &BaseDataHandler{Material: m, Supplier: su, Warehouse: w, Location: l}
}

// ---- Material ----

func (h *BaseDataHandler) MaterialList(c *gin.Context) {
	list, total, err := h.Material.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *BaseDataHandler) MaterialGet(c *gin.Context) {
	m, err := h.Material.Get(tenantOf(c), idParam(c))
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
	if err := h.Material.Create(tenantOf(c), &m); mapErr(c, err) {
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
	if err := h.Material.Update(tenantOf(c), idParam(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) MaterialDelete(c *gin.Context) {
	if mapErr(c, h.Material.Delete(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// ---- Supplier ----

func (h *BaseDataHandler) SupplierList(c *gin.Context) {
	list, total, err := h.Supplier.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *BaseDataHandler) SupplierGet(c *gin.Context) {
	m, err := h.Supplier.Get(tenantOf(c), idParam(c))
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
	if err := h.Supplier.Create(tenantOf(c), &m); mapErr(c, err) {
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
	if err := h.Supplier.Update(tenantOf(c), idParam(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) SupplierDelete(c *gin.Context) {
	if mapErr(c, h.Supplier.Delete(tenantOf(c), idParam(c))) {
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
	m, err := h.Supplier.SetAuditStatus(tenantOf(c), idParam(c), body.AuditStatus)
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

// ---- Warehouse ----

func (h *BaseDataHandler) WarehouseList(c *gin.Context) {
	list, total, err := h.Warehouse.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *BaseDataHandler) WarehouseGet(c *gin.Context) {
	m, err := h.Warehouse.Get(tenantOf(c), idParam(c))
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
	if err := h.Warehouse.Create(tenantOf(c), &m); mapErr(c, err) {
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
	if err := h.Warehouse.Update(tenantOf(c), idParam(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) WarehouseDelete(c *gin.Context) {
	if mapErr(c, h.Warehouse.Delete(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// ---- Location ----

func (h *BaseDataHandler) LocationList(c *gin.Context) {
	list, total, err := h.Location.List(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *BaseDataHandler) LocationGet(c *gin.Context) {
	m, err := h.Location.Get(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) LocationCreate(c *gin.Context) {
	var m model.Location
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.Location.Create(tenantOf(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) LocationUpdate(c *gin.Context) {
	var m model.Location
	if err := c.ShouldBindJSON(&m); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.Location.Update(tenantOf(c), idParam(c), &m); mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *BaseDataHandler) LocationDelete(c *gin.Context) {
	if mapErr(c, h.Location.Delete(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}
