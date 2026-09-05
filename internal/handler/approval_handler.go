package handler

import (
	"github.com/gin-gonic/gin"

	"scm/internal/model"
	"scm/pkg/authx"
	"scm/pkg/response"
	"scm/internal/service"
)

// ApprovalHandler exposes approval groups, the workbench and approval actions.
type ApprovalHandler struct{ svc *service.ApprovalService }

func NewApprovalHandler(s *service.ApprovalService) *ApprovalHandler {
	return &ApprovalHandler{svc: s}
}

func actorOf(c *gin.Context) *authx.Actor { return authx.GetActor(c) }

// ---- groups ----

func (h *ApprovalHandler) GroupList(c *gin.Context) {
	list, total, err := h.svc.ListGroups(tenantOf(c), parsePage(c))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}

func (h *ApprovalHandler) GroupCreate(c *gin.Context) {
	var g model.ApprovalGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	out, err := h.svc.CreateGroup(tenantOf(c), &g)
	if mapErr(c, err) {
		return
	}
	response.OK(c, out)
}

func (h *ApprovalHandler) GroupUpdate(c *gin.Context) {
	var g model.ApprovalGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	out, err := h.svc.UpdateGroup(tenantOf(c), idParam(c), &g)
	if mapErr(c, err) {
		return
	}
	response.OK(c, out)
}

func (h *ApprovalHandler) GroupDelete(c *gin.Context) {
	if mapErr(c, h.svc.DeleteGroup(tenantOf(c), idParam(c))) {
		return
	}
	response.OK(c, gin.H{"id": idParam(c)})
}

// ---- workbench ----

func (h *ApprovalHandler) Submit(c *gin.Context) {
	var body struct {
		OrderType string `json:"order_type"`
		OrderID   uint   `json:"order_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	task, err := h.svc.Submit(tenantOf(c), actorOf(c), body.OrderType, body.OrderID)
	if mapErr(c, err) {
		return
	}
	response.OK(c, task)
}

func (h *ApprovalHandler) Pending(c *gin.Context) {
	a := actorOf(c)
	list, err := h.svc.Pending(tenantOf(c), a.UserID)
	if mapErr(c, err) {
		return
	}
	response.OK(c, list)
}

func (h *ApprovalHandler) Processed(c *gin.Context) {
	a := actorOf(c)
	list, err := h.svc.Processed(tenantOf(c), a.UserID)
	if mapErr(c, err) {
		return
	}
	response.OK(c, list)
}

func (h *ApprovalHandler) Submitted(c *gin.Context) {
	a := actorOf(c)
	list, err := h.svc.Submitted(tenantOf(c), a.UserID)
	if mapErr(c, err) {
		return
	}
	response.OK(c, list)
}

func (h *ApprovalHandler) Get(c *gin.Context) {
	task, err := h.svc.GetTask(tenantOf(c), idParam(c))
	if mapErr(c, err) {
		return
	}
	response.OK(c, task)
}

func (h *ApprovalHandler) Act(c *gin.Context) {
	var body struct {
		Action  string `json:"action"`
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	a := actorOf(c)
	task, err := h.svc.Act(tenantOf(c), idParam(c), a.UserID, body.Action, body.Comment)
	if mapErr(c, err) {
		return
	}
	response.OK(c, task)
}
