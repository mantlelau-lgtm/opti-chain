package handler

import (
	"github.com/gin-gonic/gin"

	"scm/internal/pkg/authx"
	"scm/internal/pkg/response"
	"scm/internal/service"
)

// AssistantHandler exposes the in-app intelligent assistant. The chat endpoint
// is available to any authenticated user; the assistant service routes the
// question to the role-appropriate agent and enforces permissions per tool.
type AssistantHandler struct {
	svc *service.AssistantService
}

func NewAssistantHandler(s *service.AssistantService) *AssistantHandler {
	return &AssistantHandler{svc: s}
}

func (h *AssistantHandler) Chat(c *gin.Context) {
	var req struct {
		Message string `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	actor := authx.GetActor(c)
	reply, err := h.svc.Chat(c.Request.Context(), actor, req.Message)
	if mapErr(c, err) {
		return
	}
	response.OK(c, reply)
}
