// Package repository is the data-access layer. Each repository owns one
// aggregate and exposes CRUD + query operations, shielding handlers from
// GORM details. A generic Base implements the shared CRUD; concrete repos
// embed it and add domain-specific queries.
package repository

import (
	"gorm.io/gorm"

	"scm/internal/pkg/query"
)

// ListFilter is a common filter passed to list operations.
type ListFilter struct {
	Page    query.Page
	Keyword string
}

// genericRepo provides shared CRUD for any entity. It is reused via embedding
// to keep each concrete repository tiny and single-purpose.
type genericRepo[T any] struct {
	db *gormDB
}

func newGenericRepo[T any](db *gormDB) *genericRepo[T] {
	return &genericRepo[T]{db: db}
}

// Create inserts a new record.
func (r *genericRepo[T]) Create(v *T) error {
	return r.db.DB.Create(v).Error
}

// Update replaces the record identified by its primary key.
func (r *genericRepo[T]) Update(v *T) error {
	return r.db.DB.Save(v).Error
}

// Delete removes the record by id.
func (r *genericRepo[T]) Delete(id uint) error {
	var zero T
	return r.db.DB.Delete(&zero, id).Error
}

// Get returns a single record by id, or nil when not found.
func (r *genericRepo[T]) Get(id uint) (*T, error) {
	var v T
	if err := r.db.DB.First(&v, id).Error; err != nil {
		if errorsIsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

// list runs a paginated + keyword-filtered query. The apply callback customizes
// the base query (e.g. column-scoped LIKE); when nil it is ignored. It is
// unexported so concrete repos expose their own 3-arg List that delegates here.
func (r *genericRepo[T]) list(f ListFilter, apply func(*gorm.DB) *gorm.DB, out *[]T, total *int64) error {
	return paginate(r.db.DB, f, apply, out, total)
}
