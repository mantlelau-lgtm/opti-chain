package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"scm/pkg/aksk"
	"scm/pkg/authx"
	"scm/pkg/response"
)

// AuthOrKey is the authentication boundary for the protected API group. It
// accepts EITHER a Bearer JWT (browser sessions) OR the AK/SK header triple
// (agents / MCP clients). On success it stores the resolved Actor for
// downstream permission checks.
func AuthOrKey(enabled bool, parseJWT func(string) (*authx.Actor, error), verifyKey func(ak, ts, sig, method, path, bodyHash string) (*authx.Actor, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token := strings.TrimPrefix(h, "Bearer ")
			actor, err := parseJWT(token)
			if err != nil {
				unauthorized(c)
				return
			}
			authx.SetActor(c, actor)
			c.Next()
			return
		}
		ak := c.GetHeader(aksk.HeaderKey)
		ts := c.GetHeader(aksk.HeaderTimestamp)
		sig := c.GetHeader(aksk.HeaderSignature)
		if ak == "" || ts == "" || sig == "" {
			unauthorized(c)
			return
		}
		bodyHash := requestBodyHash(c)
		actor, err := verifyKey(ak, ts, sig, c.Request.Method, c.Request.URL.Path, bodyHash)
		if err != nil {
			response.HTTPFail(c, http.StatusUnauthorized, response.ErrUnauthorized, err.Error())
			c.Abort()
			return
		}
		authx.SetActor(c, actor)
		c.Next()
	}
}

// requestBodyHash reads the raw body to compute its SHA-256 (part of the AK/SK
// signature) and restores it so downstream handlers can bind normally.
func requestBodyHash(c *gin.Context) string {
	b, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(b))
	return aksk.SHA256Hex(b)
}
