package handler

import (
	"github.com/gin-gonic/gin"

	"scm/internal/pkg/response"
	"scm/internal/service"
)

// AuthHandler exposes the login endpoint.
type AuthHandler struct{ svc *service.AuthService }

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// Login verifies credentials and returns a bearer token plus a safe user
// view (no password hash).
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	token, user, err := h.svc.Login(req.Username, req.Password)
	if mapErr(c, err) {
		return
	}
	response.OK(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"name":     user.Name,
		},
	})
}
