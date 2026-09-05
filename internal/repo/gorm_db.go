package repository

import (
	"strings"

	"gorm.io/gorm"
)

// gormDB is a thin wrapper used by repositories so tests can substitute an
// in-memory store without leaking the concrete engine everywhere.
type gormDB struct {
	*gorm.DB
}

// NewGormDB wraps a *gorm.DB.
func NewGormDB(db *gorm.DB) *gormDB { return &gormDB{DB: db} }

// paginate runs a query honoring the shared list filter (page/size/keyword).
// The keyword, when set, is applied by the caller-supplied apply func; this
// keeps each repository free to define what "keyword" means for its entity.
func paginate[T any](db *gorm.DB, f ListFilter, apply func(*gorm.DB) *gorm.DB, out *[]T, total *int64) error {
	q := db.Model((*T)(nil))
	if apply != nil {
		q = apply(q)
	}
	if err := q.Count(total).Error; err != nil {
		return err
	}
	lim := f.Page.Normalize()
	return q.Offset(f.Page.Offset()).Limit(lim).Find(out).Error
}

// keywordLike builds a LIKE match across the given columns when keyword set.
// Returns nil (no extra condition) when the keyword is empty.
func keywordLike(f ListFilter, cols ...string) func(*gorm.DB) *gorm.DB {
	if f.Keyword == "" {
		return nil
	}
	return func(q *gorm.DB) *gorm.DB {
		like := "%" + f.Keyword + "%"
		conds := make([]string, 0, len(cols))
		args := make([]any, 0, len(cols))
		for _, c := range cols {
			conds = append(conds, c+" LIKE ?")
			args = append(args, like)
		}
		return q.Where(strings.Join(conds, " OR "), args...)
	}
}
