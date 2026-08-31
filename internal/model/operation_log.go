package model

// OperationLog corresponds to sys_operation_log: the audit trail of every
// successful mutation, scoped to the tenant.
type OperationLog struct {
	BaseModel
	TenantID   uint   `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	UserID     uint   `gorm:"column:user_id;index" json:"user_id"`
	Username   string `gorm:"column:username;size:64" json:"username"`
	Roles      string `gorm:"column:roles;size:128" json:"roles"`
	Module     string `gorm:"column:module;size:32;index" json:"module"`
	Action     string `gorm:"column:action;size:32;index" json:"action"`
	Resource   string `gorm:"column:resource;size:32" json:"resource"`
	ResourceID string `gorm:"column:resource_id;size:64" json:"resource_id"`
	Summary    string `gorm:"column:summary;size:255" json:"summary"`
	Method     string `gorm:"column:method;size:8" json:"method"`
	Path       string `gorm:"column:path;size:255" json:"path"`
}

func (OperationLog) TableName() string { return "sys_operation_log" }
