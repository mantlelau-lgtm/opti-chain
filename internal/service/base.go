// Package service holds business logic. Each service owns one aggregate,
// depends on a single repository, and returns domain errors that the
// handler layer maps to HTTP responses.
package service

import (
	"errors"
	"scm/pkg/query"
)

// ErrNoChange is returned by guarded updates when nothing was affected.
var ErrNoChange = errors.New("no matching record updated")

// PageInput is the canonical list request used across services.
type PageInput struct {
	Page    query.Page
	Keyword string
}
