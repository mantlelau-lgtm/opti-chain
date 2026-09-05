package handler

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"scm/pkg/authx"
	"scm/pkg/response"
	"scm/internal/service"
)

// ApiKeyHandler exposes personal AK/SK lifecycle endpoints. Every logged-in
// user issues keys for themselves: the key's permission set is derived from
// the user's roles at issuance time and the key is bound to their user id.
type ApiKeyHandler struct {
	svc  *service.ApiKeyService
	rbac *service.RBACService
}

func NewApiKeyHandler(s *service.ApiKeyService, r *service.RBACService) *ApiKeyHandler {
	return &ApiKeyHandler{svc: s, rbac: r}
}

// actor returns the authenticated principal, or nil. Key issuance requires a
// real user (JWT session), never an API-key actor (which has no user id).
func (h *ApiKeyHandler) actor(c *gin.Context) *authx.Actor {
	a := authx.GetActor(c)
	if a == nil || a.UserID == 0 {
		response.HTTPFail(c, 401, response.ErrUnauthorized, "login required")
		return nil
	}
	return a
}

func (h *ApiKeyHandler) List(c *gin.Context) {
	a := h.actor(c)
	if a == nil {
		return
	}
	list, total, err := h.svc.List(a.TenantID, a.UserID, parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *ApiKeyHandler) Create(c *gin.Context) {
	a := h.actor(c)
	if a == nil {
		return
	}
	var req struct {
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at"` // RFC3339, optional
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	var exp *time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			response.Fail(c, response.ErrBadRequest, "expires_at must be RFC3339")
			return
		}
		exp = &t
	}
	// Permission set is derived from the user's current roles (never chosen by
	// hand), so the key can only ever do what the user themselves can do.
	perms := strings.Join(h.rbac.PermsForActor(a), ",")
	key, sk, err := h.svc.CreateKey(a.TenantID, a.UserID, req.Name, perms, exp)
	if mapErr(c, err) {
		return
	}
	// The SK is returned exactly once; it cannot be recovered afterwards.
	response.OK(c, gin.H{"key": key, "sk": sk})
}

func (h *ApiKeyHandler) Disable(c *gin.Context) {
	a := h.actor(c)
	if a == nil {
		return
	}
	if mapErr(c, h.svc.Disable(a.TenantID, a.UserID, idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

func (h *ApiKeyHandler) Enable(c *gin.Context) {
	a := h.actor(c)
	if a == nil {
		return
	}
	if mapErr(c, h.svc.Enable(a.TenantID, a.UserID, idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

func (h *ApiKeyHandler) Delete(c *gin.Context) {
	a := h.actor(c)
	if a == nil {
		return
	}
	if mapErr(c, h.svc.Delete(a.TenantID, a.UserID, idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}
