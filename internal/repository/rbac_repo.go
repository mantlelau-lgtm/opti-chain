package repository

import (
	"gorm.io/gorm"

	"scm/internal/model"
)

// ---- Tenant (platform-level, not tenant-scoped) ----

type TenantRepo struct{ *genericRepo[model.Tenant] }

func NewTenantRepo(db *gormDB) *TenantRepo {
	return &TenantRepo{genericRepo: newGenericRepo[model.Tenant](db)}
}

func (r *TenantRepo) GetByCode(code string) (*model.Tenant, error) {
	var t model.Tenant
	if err := r.db.DB.Where("code = ?", code).First(&t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (r *TenantRepo) List(f ListFilter, out *[]model.Tenant, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("code LIKE ? OR name LIKE ?", like, like)
		}
		return q.Order("id")
	}
	return r.list(f, apply, out, total)
}

// ---- Role / Module / Permission catalogs (platform-level) ----

type RoleRepo struct{ *genericRepo[model.Role] }

func NewRoleRepo(db *gormDB) *RoleRepo {
	return &RoleRepo{genericRepo: newGenericRepo[model.Role](db)}
}

func (r *RoleRepo) All() ([]model.Role, error) {
	var out []model.Role
	err := r.db.DB.Order("id").Find(&out).Error
	return out, err
}

type ModuleRepo struct{ *genericRepo[model.Module] }

func NewModuleRepo(db *gormDB) *ModuleRepo {
	return &ModuleRepo{genericRepo: newGenericRepo[model.Module](db)}
}

func (r *ModuleRepo) All() ([]model.Module, error) {
	var out []model.Module
	err := r.db.DB.Order("sort, id").Find(&out).Error
	return out, err
}

type PermissionRepo struct{ *genericRepo[model.Permission] }

func NewPermissionRepo(db *gormDB) *PermissionRepo {
	return &PermissionRepo{genericRepo: newGenericRepo[model.Permission](db)}
}

func (r *PermissionRepo) All() ([]model.Permission, error) {
	var out []model.Permission
	err := r.db.DB.Order("module_id, id").Find(&out).Error
	return out, err
}

// CodesForRoles returns the distinct permission codes granted to the roles.
func (r *PermissionRepo) CodesForRoles(roleIDs []uint) ([]string, error) {
	var codes []string
	err := r.db.DB.Model(&model.Permission{}).
		Joins("JOIN sys_role_permission rp ON rp.permission_id = sys_permission.id").
		Where("rp.role_id IN ?", roleIDs).
		Distinct("sys_permission.code").
		Pluck("sys_permission.code", &codes).Error
	return codes, err
}

// ---- Assignments ----

type UserRoleRepo struct{ *genericRepo[model.UserRole] }

func NewUserRoleRepo(db *gormDB) *UserRoleRepo {
	return &UserRoleRepo{genericRepo: newGenericRepo[model.UserRole](db)}
}

// RoleIDsForUser lists the role ids assigned to a user.
func (r *UserRoleRepo) RoleIDsForUser(userID uint) ([]uint, error) {
	var ids []uint
	err := r.db.DB.Model(&model.UserRole{}).
		Where("user_id = ?", userID).Pluck("role_id", &ids).Error
	return ids, err
}

// RoleCodesForUser resolves a user's role codes (for the JWT).
func (r *UserRoleRepo) RoleCodesForUser(userID uint) ([]string, error) {
	var codes []string
	err := r.db.DB.Model(&model.UserRole{}).
		Joins("JOIN sys_role r ON r.id = sys_user_role.role_id").
		Where("sys_user_role.user_id = ?", userID).
		Pluck("r.code", &codes).Error
	return codes, err
}

// SetRoles replaces a user's role assignments in one transaction.
func (r *UserRoleRepo) SetRoles(userID uint, roleIDs []uint) error {
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserRole{}).Error; err != nil {
			return err
		}
		for _, rid := range roleIDs {
			if err := tx.Create(&model.UserRole{UserID: userID, RoleID: rid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
