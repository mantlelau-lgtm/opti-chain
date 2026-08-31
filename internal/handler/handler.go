// Package handler is the HTTP layer. Each handler maps HTTP requests to service
// calls and results to the uniform response envelope. Handlers hold no business
// logic; they only translate I/O, keeping the layer single-responsibility.
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"scm/internal/pkg/authx"
	"scm/internal/pkg/query"
	"scm/internal/pkg/response"
	"scm/internal/service"
)

// tenantOf resolves the tenant for data access: the authenticated actor's
// tenant, or the seeded default tenant (id 1) when auth is disabled in dev.
func tenantOf(c *gin.Context) uint {
	if a := authx.GetActor(c); a != nil && a.TenantID != 0 {
		return a.TenantID
	}
	return 1
}

// parsePage reads pagination from the query string.
func parsePage(c *gin.Context) PageInput {
	var p query.Page
	c.ShouldBindQuery(&p)
	return PageInput{Page: p, Keyword: c.Query("keyword")}
}

// PageInput aliases the service-level list request.
type PageInput = service.PageInput

// idParam parses the :id path segment.
func idParam(c *gin.Context) uint {
	v, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	return uint(v)
}

// mapErr translates a service error into a response envelope. It returns true
// when an error was written, signalling the handler to stop.
func mapErr(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, service.ErrBadRequest) {
		response.Fail(c, response.ErrBadRequest, err.Error())
		return true
	}
	if errors.Is(err, service.ErrUnauthorized) {
		response.HTTPFail(c, 401, response.ErrUnauthorized, err.Error())
		return true
	}
	if errors.Is(err, service.ErrForbidden) {
		response.HTTPFail(c, 403, response.ErrForbidden, err.Error())
		return true
	}
	if errors.Is(err, service.ErrNotFound) {
		response.HTTPFail(c, 404, response.ErrNotFound, err.Error())
		return true
	}
	if errors.Is(err, service.ErrConflict) {
		response.HTTPFail(c, 409, response.ErrConflict, err.Error())
		return true
	}
	response.HTTPFail(c, 500, response.ErrInternal, err.Error())
	return true
}
