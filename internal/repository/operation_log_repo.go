package repository

import (
	"gorm.io/gorm"

	"scm/internal/model"
)

// OperationLogRepo owns sys_operation_log (append-only audit trail).
type OperationLogRepo struct{ *genericRepo[model.OperationLog] }

func NewOperationLogRepo(db *gormDB) *OperationLogRepo {
	return &OperationLogRepo{genericRepo: newGenericRepo[model.OperationLog](db)}
}

// AuditFilter scopes an audit-log search.
type AuditFilter struct {
	TenantID uint   // 0 = all tenants (platform view); >0 = one tenant
	User     string // username keyword
	Role     string // role code (LIKE)
	Module   string
	Action   string
	DateFrom string // YYYY-MM-DD
	DateTo   string // YYYY-MM-DD
}

// Search lists logs with the given filters, newest first. TenantID == 0 means
// no tenant scoping (platform).
func (r *OperationLogRepo) Search(f ListFilter, af AuditFilter, out *[]model.OperationLog, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if af.TenantID != 0 {
			q = q.Where("tenant_id = ?", af.TenantID)
		}
		if af.User != "" {
			q = q.Where("username LIKE ?", "%"+af.User+"%")
		}
		if af.Role != "" {
			q = q.Where("roles LIKE ?", "%"+af.Role+"%")
		}
		if af.Module != "" {
			q = q.Where("module = ?", af.Module)
		}
		if af.Action != "" {
			q = q.Where("action = ?", af.Action)
		}
		if af.DateFrom != "" {
			q = q.Where("created_at >= ?", af.DateFrom+" 00:00:00")
		}
		if af.DateTo != "" {
			q = q.Where("created_at <= ?", af.DateTo+" 23:59:59")
		}
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("summary LIKE ? OR path LIKE ?", like, like)
		}
		return q.Order("id DESC")
	}
	return paginate(r.db.DB, f, apply, out, total)
}
