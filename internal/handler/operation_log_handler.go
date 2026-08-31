package handler

import (
	"github.com/gin-gonic/gin"

	"scm/internal/pkg/response"
	"scm/internal/repository"
	"scm/internal/service"
)

// OperationLogHandler exposes the audit trail. Business tenants see their own
// logs; the platform tenant may search across all tenants.
type OperationLogHandler struct{ svc *service.AuditService }

func NewOperationLogHandler(s *service.AuditService) *OperationLogHandler {
	return &OperationLogHandler{svc: s}
}

// List filters by tenant(platform only)/user/role/module/action/date range.
func (h *OperationLogHandler) List(c *gin.Context) {
	var tenantID uint
	if v := c.Query("tenant_id"); v != "" {
		tenantID = parseUint(v)
	}
	af := repository.AuditFilter{
		TenantID: tenantID,
		User:     c.Query("user"),
		Role:     c.Query("role"),
		Module:   c.Query("module"),
		Action:   c.Query("action"),
		DateFrom: c.Query("date_from"),
		DateTo:   c.Query("date_to"),
	}
	list, total, err := h.svc.Search(tenantOf(c), parsePage(c), af)
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}
