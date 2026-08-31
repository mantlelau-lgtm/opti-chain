package repository

import (
	"gorm.io/gorm"

	"scm/internal/model"
)

// DataSourceRepo owns sys_data_source (platform-level storage targets).
type DataSourceRepo struct{ *genericRepo[model.DataSource] }

func NewDataSourceRepo(db *gormDB) *DataSourceRepo {
	return &DataSourceRepo{genericRepo: newGenericRepo[model.DataSource](db)}
}

func (r *DataSourceRepo) List(f ListFilter, out *[]model.DataSource, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("name LIKE ?", like)
		}
		return q.Order("id")
	}
	return r.list(f, apply, out, total)
}
