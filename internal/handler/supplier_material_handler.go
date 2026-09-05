package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"scm/pkg/response"
	"scm/internal/service"
)

func parseUint(s string) uint {
	v, _ := strconv.ParseUint(s, 10, 64)
	return uint(v)
}

// SupplierMaterialHandler exposes the supplier↔material supply relationships.
type SupplierMaterialHandler struct{ svc *service.SupplierMaterialService }

func NewSupplierMaterialHandler(s *service.SupplierMaterialService) *SupplierMaterialHandler {
	return &SupplierMaterialHandler{svc: s}
}

// List filters by supplier_id and/or material_id (both optional).
func (h *SupplierMaterialHandler) List(c *gin.Context) {
	var supplierID, materialID uint
	if v := c.Query("supplier_id"); v != "" {
		supplierID = parseUint(v)
	}
	if v := c.Query("material_id"); v != "" {
		materialID = parseUint(v)
	}
	list, err := h.svc.List(tenantOf(c), supplierID, materialID)
	if mapErr(c, err) {
		return
	}
	response.OK(c, list)
}

func (h *SupplierMaterialHandler) Bind(c *gin.Context) {
	var in service.BindInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	m, err := h.svc.Bind(tenantOf(c), in)
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *SupplierMaterialHandler) Update(c *gin.Context) {
	var in service.BindInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	m, err := h.svc.Update(tenantOf(c), idParam(c), in)
	if mapErr(c, err) {
		return
	}
	response.OK(c, m)
}

func (h *SupplierMaterialHandler) Unbind(c *gin.Context) {
	if mapErr(c, h.svc.Unbind(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}
