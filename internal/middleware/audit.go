package middleware

import (
	"bytes"
	"encoding/json"

	"github.com/gin-gonic/gin"

	"scm/pkg/authx"
	"scm/internal/service"
)

// captureWriter tees the response body so the audit middleware can inspect the
// envelope code after the handler has written.
type captureWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *captureWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Audit logs every successful mutation (POST/PUT/DELETE whose envelope code is
// 0) to the operation log. Reads and failed requests are ignored.
func Audit(enabled bool, svc *service.AuditService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled || c.Request.Method == "GET" {
			c.Next()
			return
		}
		cw := &captureWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = cw
		c.Next()

		var env struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(cw.body.Bytes(), &env); err != nil || env.Code != 0 {
			return
		}
		svc.Log(authx.GetActor(c), c.Request.Method, c.Request.URL.Path)
	}
}
