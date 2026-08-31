package handler

import (
	"github.com/gin-gonic/gin"

	"scm/internal/model"
	"scm/internal/pkg/authx"
	"scm/internal/pkg/response"
	"scm/internal/service"
)

// RBACHandler exposes tenant/user management and the auth introspection
// endpoints used by the SPA to build its menu.
type RBACHandler struct {
	svc  *service.RBACService
	auth *service.AuthService
}

func NewRBACHandler(s *service.RBACService, a *service.AuthService) *RBACHandler {
	return &RBACHandler{svc: s, auth: a}
}

// Login verifies tenant + credentials and returns token + user + roles.
func (h *RBACHandler) Login(c *gin.Context) {
	var req struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		TenantCode string `json:"tenant_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	token, user, roles, err := h.auth.Login(req.Username, req.Password, req.TenantCode)
	if mapErr(c, err) {
		return
	}
	response.OK(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"name":     user.Name,
			"tenant":   req.TenantCode,
		},
		"roles": roles,
	})
}

// Me returns the actor plus the permission codes for menu gating.
func (h *RBACHandler) Me(c *gin.Context) {
	a := authx.GetActor(c)
	response.OK(c, gin.H{
		"user":  gin.H{"id": a.UserID, "username": a.Username, "name": a.Name, "tenant_id": a.TenantID},
		"roles": a.Roles,
		"perms": h.svc.PermsForActor(a),
	})
}

func (h *RBACHandler) Catalog(c *gin.Context) {
	out, err := h.svc.Catalog()
	if mapErr(c, err) {
		return
	}
	response.OK(c, out)
}

// ---- tenants (platform scope) ----

func (h *RBACHandler) TenantList(c *gin.Context) {
	list, total, err := h.svc.ListTenants(parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *RBACHandler) TenantCreate(c *gin.Context) {
	var t model.Tenant
	if err := c.ShouldBindJSON(&t); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.svc.CreateTenant(&t); mapErr(c, err) {
		return
	}
	response.OK(c, t)
}

func (h *RBACHandler) TenantUpdate(c *gin.Context) {
	var t model.Tenant
	if err := c.ShouldBindJSON(&t); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if err := h.svc.UpdateTenant(idParam(c), &t); mapErr(c, err) {
		return
	}
	response.OK(c, t)
}

// ---- users (tenant scope) ----

func (h *RBACHandler) UserList(c *gin.Context) {
	list, total, err := h.svc.ListUsers(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *RBACHandler) UserCreate(c *gin.Context) {
	var in service.CreateUserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	u, err := h.svc.CreateUser(tenantOf(c), in)
	if mapErr(c, err) {
		return
	}
	response.OK(c, u)
}

func (h *RBACHandler) UserUpdate(c *gin.Context) {
	var in service.CreateUserInput
	if err := c.ShouldBindJSON(&in); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	u, err := h.svc.UpdateUser(tenantOf(c), idParam(c), in)
	if mapErr(c, err) {
		return
	}
	response.OK(c, u)
}

func (h *RBACHandler) UserDelete(c *gin.Context) {
	if mapErr(c, h.svc.DeleteUser(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

func (h *RBACHandler) UserRoles(c *gin.Context) {
	roles, err := h.svc.UserRoles(idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, roles)
}
