package repository

import (
	"gorm.io/gorm"

	"scm/internal/model"
)

// UserRepo owns sys_user. Users belong to a tenant; usernames are unique per
// tenant (composite index enforced by migration SQL).
type UserRepo struct{ *genericRepo[model.User] }

func NewUserRepo(db *gormDB) *UserRepo {
	return &UserRepo{genericRepo: newGenericRepo[model.User](db)}
}

// GetByTenantUsername looks a user up within a tenant; nil when absent.
func (r *UserRepo) GetByTenantUsername(t uint, username string) (*model.User, error) {
	var u model.User
	if err := r.db.DB.Where("tenant_id = ? AND username = ?", t, username).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// Get returns one user scoped to the tenant, nil when absent.
func (r *UserRepo) Get(t, id uint) (*model.User, error) {
	var u model.User
	if err := r.db.DB.Where("tenant_id = ?", t).First(&u, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

// Create inserts a user (tenant stamped on the model by the caller).
func (r *UserRepo) Create(u *model.User) error {
	return r.db.DB.Create(u).Error
}

// Update persists a user within the tenant.
func (r *UserRepo) Update(t uint, u *model.User) error {
	u.TenantID = t
	return r.db.DB.Save(u).Error
}

// Delete removes a user within the tenant.
func (r *UserRepo) Delete(t, id uint) error {
	return r.db.DB.Where("tenant_id = ?", t).Delete(&model.User{}, id).Error
}

// List returns paginated users of the tenant.
func (r *UserRepo) List(t uint, f ListFilter, out *[]model.User, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		q = q.Where("tenant_id = ?", t)
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("username LIKE ? OR name LIKE ?", like, like)
		}
		return q.Order("id")
	}
	return paginate(r.db.DB, f, apply, out, total)
}
