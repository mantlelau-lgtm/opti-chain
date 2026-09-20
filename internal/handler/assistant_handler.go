package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"scm/internal/memory"
	"scm/internal/service"
	"scm/pkg/authx"
	"scm/pkg/response"
)

const (
	maxAttachmentSize = 5 << 20 // 5 MB
	imageMaxSize      = 5 << 20
)

type AssistantHandler struct {
	svc       *service.AssistantService
	memory    *memory.Service
	uploadDir string
}

func NewAssistantHandler(s *service.AssistantService, m *memory.Service) *AssistantHandler {
	dir := os.Getenv("SCM_UPLOAD_DIR")
	if dir == "" {
		dir = filepath.Join(".", "uploads", "assistant")
	}
	_ = os.MkdirAll(dir, 0o755)
	return &AssistantHandler{svc: s, memory: m, uploadDir: dir}
}

type attachmentInput struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MIME     string `json:"mime"`
	StoredAt string `json:"stored_at"`
	Size     int64  `json:"size"`
}

func (h *AssistantHandler) Upload(c *gin.Context) {
	actor := authx.GetActor(c)
	if actor == nil || actor.UserID == 0 {
		response.HTTPFail(c, 401, response.ErrUnauthorized, "login required")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Fail(c, response.ErrBadRequest, "file is required: "+err.Error())
		return
	}
	defer file.Close()
	if header.Size > maxAttachmentSize {
		response.Fail(c, response.ErrBadRequest,
			fmt.Sprintf("file too large: %d bytes > %d bytes (5 MB)", header.Size, maxAttachmentSize))
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExt := map[string]bool{
		".txt": true, ".md": true, ".csv": true, ".json": true, ".log": true, ".markdown": true, ".tsv": true,
		".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".bmp": true,
		".pdf": true, ".docx": true, ".xlsx": true, ".xls": true,
	}
	if !allowedExt[ext] {
		response.Fail(c, response.ErrBadRequest, "unsupported file type: "+ext)
		return
	}
	id := uuid.NewString()
	stored := filepath.Join(h.uploadDir, fmt.Sprintf("%s_%s%s", time.Now().Format("20060102"), id, ext))
	out, err := os.Create(stored)
	if err != nil {
		response.Fail(c, response.ErrInternal, "save file failed: "+err.Error())
		return
	}
	defer out.Close()
	size, err := io.Copy(out, file)
	if err != nil {
		response.Fail(c, response.ErrInternal, "save file failed: "+err.Error())
		return
	}
	mime := header.Header.Get("Content-Type")
	if mime == "" {
		mime = detectMimeByExt(ext)
	}
	response.OK(c, attachmentInput{
		ID:       id,
		Name:     header.Filename,
		MIME:     mime,
		StoredAt: stored,
		Size:     size,
	})
}

func detectMimeByExt(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".md":
		return "text/markdown"
	case ".csv":
		return "text/csv"
	case ".json":
		return "application/json"
	default:
		return "text/plain"
	}
}

func (h *AssistantHandler) Chat(c *gin.Context) {
	var req struct {
		Message     string                  `json:"message"`
		Attachments []attachmentInput       `json:"attachments"`
		Files       []*multipart.FileHeader `form:"files"`
	}
	useMultipart := strings.HasPrefix(c.ContentType(), "multipart/form-data")
	var attachments []attachmentInput
	if useMultipart {
		if err := c.Request.ParseMultipartForm(int64(maxAttachmentSize * 4)); err != nil {
			response.Fail(c, response.ErrBadRequest, "parse form failed: "+err.Error())
			return
		}
		req.Message = c.PostForm("message")
		if raw := c.PostForm("attachments"); raw != "" {
			_ = gin.H{}
		}
		files := c.Request.MultipartForm.File["files"]
		for _, fh := range files {
			if fh.Size > maxAttachmentSize {
				response.Fail(c, response.ErrBadRequest,
					fmt.Sprintf("file %s too large (%d > %d bytes)", fh.Filename, fh.Size, maxAttachmentSize))
				return
			}
			att, err := h.saveUploaded(fh)
			if err != nil {
				response.Fail(c, response.ErrInternal, err.Error())
				return
			}
			attachments = append(attachments, att)
		}
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Fail(c, response.ErrBadRequest, err.Error())
			return
		}
		attachments = req.Attachments
	}
	actor := authx.GetActor(c)
	svcAtts := make([]service.Attachment, 0, len(attachments))
	for _, a := range attachments {
		svcAtts = append(svcAtts, service.Attachment{
			ID:       a.ID,
			Name:     a.Name,
			MIME:     a.MIME,
			StoredAt: a.StoredAt,
			Size:     a.Size,
		})
	}
	reply, err := h.svc.ChatWithAttachments(c.Request.Context(), actor, req.Message, svcAtts)
	if mapErr(c, err) {
		return
	}
	// Persist the turn to short-term memory. Missing/empty memory is ignored
	// (best-effort) so a memory misconfig never breaks the chat UI.
	if h.memory != nil && actor != nil && actor.UserID != 0 {
		userAttsJSON := ""
		if len(attachments) > 0 {
			if b, e := json.Marshal(attachments); e == nil {
				userAttsJSON = string(b)
			}
		}
		usageJSON := ""
		if reply.Usage.TotalTokens > 0 || reply.Usage.TotalMs > 0 || reply.Usage.Model != "" {
			if b, e := json.Marshal(reply.Usage); e == nil {
				usageJSON = string(b)
			}
		}
		toolCallsStr := ""
		if len(reply.ToolCalls) > 0 {
			if b, e := json.Marshal(reply.ToolCalls); e == nil {
				toolCallsStr = string(b)
			}
		} else if len(reply.ToolCalls) == 0 {
			// nil slice → preserve empty rather than marshalling "null"
		}
		h.memory.Store(c.Request.Context(), actor,
			reply.Agent, reply.AgentName,
			req.Message, userAttsJSON,
			reply.Reply, usageJSON, toolCallsStr)
	}
	// Echo the stored attachments metadata back to the frontend so it can
	// update the just-sent user message with persistent (non-blob) URLs
	// without waiting for a page refresh.
	out := struct {
		*service.AssistantReply
		UserAttachments []attachmentInput `json:"user_attachments,omitempty"`
	}{
		AssistantReply:  reply,
		UserAttachments: attachments,
	}
	response.OK(c, out)
}

// DownloadAttachment serves a previously-uploaded assistant file by its
// stored-at path. The path is restricted to the configured upload dir to
// prevent directory traversal. Only the uploader can download (enforced by
// JWT auth; per-user isolation is best-effort because files are readable by
// anyone with the full path).
func (h *AssistantHandler) DownloadAttachment(c *gin.Context) {
	actor := authx.GetActor(c)
	if actor == nil || actor.UserID == 0 {
		response.HTTPFail(c, 401, response.ErrUnauthorized, "login required")
		return
	}
	storedAt := c.Query("path")
	if storedAt == "" {
		storedAt = c.Param("path")
	}
	if storedAt == "" {
		response.Fail(c, response.ErrBadRequest, "path is required")
		return
	}
	absUploadDir, err := filepath.Abs(h.uploadDir)
	if err != nil {
		response.Fail(c, response.ErrInternal, "upload dir invalid")
		return
	}
	cleanPath := filepath.Clean(storedAt)
	absReq, err := filepath.Abs(cleanPath)
	if err != nil {
		response.Fail(c, response.ErrBadRequest, "invalid path")
		return
	}
	rel, err := filepath.Rel(absUploadDir, absReq)
	if err != nil || strings.HasPrefix(rel, "..") || strings.HasPrefix(rel, "."+string(filepath.Separator)) {
		response.HTTPFail(c, 403, response.ErrForbidden, "path outside upload dir")
		return
	}
	info, err := os.Stat(absReq)
	if err != nil || info.IsDir() {
		response.HTTPFail(c, 404, response.ErrNotFound, "file not found")
		return
	}
	c.File(absReq)
}

func (h *AssistantHandler) saveUploaded(fh *multipart.FileHeader) (attachmentInput, error) {
	src, err := fh.Open()
	if err != nil {
		return attachmentInput{}, fmt.Errorf("open file: %w", err)
	}
	defer src.Close()
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	id := uuid.NewString()
	stored := filepath.Join(h.uploadDir, fmt.Sprintf("%s_%s%s", time.Now().Format("20060102"), id, ext))
	out, err := os.Create(stored)
	if err != nil {
		return attachmentInput{}, fmt.Errorf("create file: %w", err)
	}
	defer out.Close()
	size, err := io.Copy(out, src)
	if err != nil {
		return attachmentInput{}, fmt.Errorf("write file: %w", err)
	}
	mime := fh.Header.Get("Content-Type")
	if mime == "" {
		mime = detectMimeByExt(ext)
	}
	return attachmentInput{
		ID:       id,
		Name:     fh.Filename,
		MIME:     mime,
		StoredAt: stored,
		Size:     size,
	}, nil
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
	// Optional ?limit=N caps the replay window; absent/invalid falls back to
	// the configured default. History is read straight from the JSONL log, so
	// the UI is no longer bounded by the LLM context window.
	limit, _ := strconv.Atoi(c.Query("limit"))
	response.OK(c, gin.H{"history": h.memory.History(c.Request.Context(), actor, limit)})
}
