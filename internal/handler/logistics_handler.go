package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"scm/internal/service"
	"scm/pkg/authx"
	"scm/pkg/response"
)

type LogisticsHandler struct {
	svc *service.LogisticsService
}

func NewLogisticsHandler(s *service.LogisticsService) *LogisticsHandler {
	return &LogisticsHandler{svc: s}
}

func (h *LogisticsHandler) Query(c *gin.Context) {
	actor := authx.GetActor(c)
	var req struct {
		TrackingNo string `json:"tracking_no"`
		PhoneNo    string `json:"phone_no"` // optional: last 4 digits
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.TrackingNo) == "" {
		response.Fail(c, response.ErrBadRequest, "tracking_no is required")
		return
	}
	res, err := h.svc.QueryOne(c.Request.Context(), actor.TenantID, actor.UserID, strings.TrimSpace(req.TrackingNo))
	if mapErr(c, err) {
		return
	}
	response.OK(c, res)
}

func (h *LogisticsHandler) Upload(c *gin.Context) {
	actor := authx.GetActor(c)
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		response.Fail(c, response.ErrBadRequest, "file is required")
		return
	}
	defer file.Close()
	buf := make([]byte, 1024*1024)
	n, _ := file.Read(buf)
	content := string(buf[:n])

	// Parse tracking numbers from file content (split by newline/comma/tab)
	var trackingNos []string
	for _, line := range strings.Split(content, "\n") {
		for _, token := range strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == '\t' || r == ' ' }) {
			token = strings.TrimSpace(token)
			if token != "" {
				trackingNos = append(trackingNos, token)
			}
		}
	}
	if len(trackingNos) == 0 {
		response.Fail(c, response.ErrBadRequest, "no tracking numbers found in file")
		return
	}

	phoneNo := c.PostForm("phone_no")
	res, err := h.svc.QueryBatch(c.Request.Context(), actor.TenantID, actor.UserID, trackingNos, 1, phoneNo)
	if mapErr(c, err) {
		return
	}
	response.OK(c, res)
}

func (h *LogisticsHandler) List(c *gin.Context) {
	actor := authx.GetActor(c)
	list, total, err := h.svc.List(c.Request.Context(), actor.TenantID, actor.UserID, parseIntDefault(c.Query("page"), 1), parseIntDefault(c.Query("size"), 20))
	if mapErr(c, err) {
		return
	}
	response.OKPage(c, total, list)
}