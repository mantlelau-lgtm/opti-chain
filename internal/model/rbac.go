package model

// Fixed role codes (v0.2 decision: six roles, seeded, not user-configurable).
const (
	RoleAdmin         = "admin"
	RoleCategoryMgr   = "category_manager"
	RoleProcAssistant = "procurement_assistant"
	RoleCommittee     = "committee"
	RoleQCWH          = "qc_wh"
	RoleFinance       = "finance"
)

// Module corresponds to sys_module: a first-class menu/domain grouping so the
// module catalog lives in tables, not code.
type Module struct {
	BaseModel
	Code string `gorm:"column:code;size:64;uniqueIndex;not null" json:"code"`
	Name string `gorm:"column:name;size:64;not null" json:"name"`
	Sort int    `gorm:"column:sort;default:0" json:"sort"`
}

func (Module) TableName() string { return "sys_module" }

// Permission corresponds to sys_permission: one actionable point
// (resource:action) bound to a module, with the HTTP route it guards stored
// for reference/admin UI.
type Permission struct {
	BaseModel
	Code     string `gorm:"column:code;size:64;uniqueIndex;not null" json:"code"`
	Name     string `gorm:"column:name;size:64;not null" json:"name"`
	ModuleID uint   `gorm:"column:module_id;not null;index" json:"module_id"`
	Method   string `gorm:"column:method;size:8" json:"method"`
	Route    string `gorm:"column:route;size:128" json:"route"`
}

func (Permission) TableName() string { return "sys_permission" }

// Role corresponds to sys_role: global (platform-level) seeded roles.
type Role struct {
	BaseModel
	Code   string `gorm:"column:code;size:64;uniqueIndex;not null" json:"code"`
	Name   string `gorm:"column:name;size:64;not null" json:"name"`
	Remark string `gorm:"column:remark;size:255" json:"remark"`
}

func (Role) TableName() string { return "sys_role" }

// UserRole assigns roles to users (users are already tenant-scoped).
type UserRole struct {
	BaseModel
	UserID uint `gorm:"column:user_id;not null;uniqueIndex:idx_user_role" json:"user_id"`
	RoleID uint `gorm:"column:role_id;not null;uniqueIndex:idx_user_role" json:"role_id"`
}

func (UserRole) TableName() string { return "sys_user_role" }

// RolePermission is the role → permission matrix (seeded per RACI).
type RolePermission struct {
	BaseModel
	RoleID       uint `gorm:"column:role_id;not null;uniqueIndex:idx_role_perm" json:"role_id"`
	PermissionID uint `gorm:"column:permission_id;not null;uniqueIndex:idx_role_perm" json:"permission_id"`
}

func (RolePermission) TableName() string { return "sys_role_permission" }
