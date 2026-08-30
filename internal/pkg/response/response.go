// Package response defines the uniform API envelope used by all HTTP handlers.
// Keeping a single response contract here means front and back stay in sync.
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Body is the standard JSON envelope: {code, message, data}.
type Body struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageData is a paginated list payload.
type PageData struct {
	Total int64       `json:"total"`
	List  interface{} `json:"list"`
}

// OK writes a 200 success with data.
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "ok", Data: data})
}

// OKPage writes a 200 success with a paginated payload.
func OKPage(c *gin.Context, total int64, list interface{}) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "ok", Data: PageData{Total: total, List: list}})
}

// Fail writes a non-zero business code with an error message.
func Fail(c *gin.Context, code int, msg string) {
	c.JSON(http.StatusOK, Body{Code: code, Message: msg})
}

// HTTPFail writes an HTTP error status (used for 4xx/5xx).
func HTTPFail(c *gin.Context, status, code int, msg string) {
	c.JSON(status, Body{Code: code, Message: msg})
}

// Business error codes.
const (
	ErrBadRequest = 40000
	ErrNotFound   = 40400
	ErrConflict   = 40900
	ErrInternal   = 50000
)
