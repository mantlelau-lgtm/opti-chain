package handler

import (
	"github.com/gin-gonic/gin"

	"scm/internal/model"
	"scm/internal/pkg/response"
	"scm/internal/service"
)

// StorageHandler exposes data-source configuration and storage migration.
// All endpoints are platform-only.
type StorageHandler struct {
	svc        *service.StorageService
	isPlatform func(uint) bool
}

func NewStorageHandler(s *service.StorageService, isPlatform func(uint) bool) *StorageHandler {
	return &StorageHandler{svc: s, isPlatform: isPlatform}
}

func (h *StorageHandler) requirePlatform(c *gin.Context) bool {
	if h.isPlatform(tenantOf(c)) {
		return true
	}
	response.HTTPFail(c, 403, response.ErrForbidden, "platform only")
	return false
}

func (h *StorageHandler) DataSourceList(c *gin.Context) {
	if !h.requirePlatform(c) {
		return
	}
	list, total, err := h.svc.ListDataSources(parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *StorageHandler) DataSourceCreate(c *gin.Context) {
	if !h.requirePlatform(c) {
		return
	}
	var ds model.DataSource
	if err := c.ShouldBindJSON(&ds); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.svc.CreateDataSource(&ds); mapErr(c, err) {
		return
	}
	response.OK(c, ds)
}

func (h *StorageHandler) DataSourceDelete(c *gin.Context) {
	if !h.requirePlatform(c) {
		return
	}
	if mapErr(c, h.svc.DeleteDataSource(idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

func (h *StorageHandler) TestConnection(c *gin.Context) {
	if !h.requirePlatform(c) {
		return
	}
	var body struct {
		Driver string `json:"driver"`
		DSN    string `json:"dsn"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.svc.TestConnection(body.Driver, body.DSN); mapErr(c, err) {
		return
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *StorageHandler) Migrate(c *gin.Context) {
	if !h.requirePlatform(c) {
		return
	}
	if err := h.svc.Migrate(idParam(c)); mapErr(c, err) {
		return
	}
	response.OK(c, gin.H{"started": true})
}

func (h *StorageHandler) Status(c *gin.Context) {
	if !h.requirePlatform(c) {
		return
	}
	response.OK(c, h.svc.Status())
}
