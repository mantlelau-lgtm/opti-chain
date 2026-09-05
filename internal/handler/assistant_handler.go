package handler

import (
	"github.com/gin-gonic/gin"

	"scm/internal/memory"
	"scm/internal/service"
	"scm/pkg/authx"
	"scm/pkg/response"
)

// AssistantHandler exposes the in-app intelligent assistant. The chat endpoint
// is available to any authenticated user; the assistant service routes the
// question to the role-appropriate agent and enforces permissions per tool.
type AssistantHandler struct {
	svc    *service.AssistantService
	memory *memory.Service
}

func NewAssistantHandler(s *service.AssistantService, m *memory.Service) *AssistantHandler {
	return &AssistantHandler{svc: s, memory: m}
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

func (h *AssistantHandler) ClearMemory(c *gin.Context) {
	actor := authx.GetActor(c)
	if actor == nil || actor.UserID == 0 {
		response.HTTPFail(c, 401, response.ErrUnauthorized, "login required")
		return
	}
	if h.memory != nil {
		_ = h.memory.Clear(c.Request.Context(), actor)
	}
	response.OK(c, gin.H{"ok": true})
}

func (h *AssistantHandler) GetHistory(c *gin.Context) {
	actor := authx.GetActor(c)
	if actor == nil || actor.UserID == 0 {
		response.HTTPFail(c, 401, response.ErrUnauthorized, "login required")
		return
	}
	if h.memory == nil {
		response.OK(c, gin.H{"history": []any{}})
		return
	}
	result := h.memory.Retrieve(c.Request.Context(), actor)
	response.OK(c, gin.H{"history": result.ShortTerm})
}