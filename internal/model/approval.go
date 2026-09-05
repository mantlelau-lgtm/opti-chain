package model

import "time"

// Approval order types.
const (
	ApprovalTypePO = "PO"
	ApprovalTypeSO = "SO"
)

// Approval modes.
const (
	ApprovalModeAll = "ALL" // 会签：全员通过
	ApprovalModeAny = "ANY" // 或签：任一通过
)

// Approval statuses.
const (
	ApprovalStatusPending  = "PENDING"
	ApprovalStatusApproved = "APPROVED"
	ApprovalStatusRejected = "REJECTED"
)

// ApprovalGroup corresponds to sys_approval_group: a tenant-level group of
// approvers for a given order type.
type ApprovalGroup struct {
	TenantBaseModel
	Name      string                `gorm:"column:name;size:64;not null" json:"name"`
	OrderType string                `gorm:"column:order_type;size:16;not null" json:"order_type"`
	Mode      string                `gorm:"column:mode;size:16;not null;default:ALL" json:"mode"`
	Members   []ApprovalGroupMember `gorm:"foreignKey:GroupID" json:"members,omitempty"`
}

func (ApprovalGroup) TableName() string { return "sys_approval_group" }

// ApprovalGroupMember corresponds to sys_approval_group_member.
type ApprovalGroupMember struct {
	BaseModel
	GroupID uint `gorm:"column:group_id;not null;index" json:"group_id"`
	UserID  uint `gorm:"column:user_id;not null;index" json:"user_id"`
}

func (ApprovalGroupMember) TableName() string { return "sys_approval_group_member" }

// ApprovalTask corresponds to sys_approval_task: one approval flow instance
// for a submitted order.
type ApprovalTask struct {
	TenantBaseModel
	OrderType     string               `gorm:"column:order_type;size:16;not null;index" json:"order_type"`
	OrderID       uint                 `gorm:"column:order_id;not null;index" json:"order_id"`
	OrderNumber   string               `gorm:"column:order_number;size:64" json:"order_number"`
	Status        string               `gorm:"column:status;size:16;not null;default:PENDING" json:"status"`
	SubmitterID   uint                 `gorm:"column:submitter_id" json:"submitter_id"`
	SubmitterName string               `gorm:"column:submitter_name;size:64" json:"submitter_name"`
	Members       []ApprovalTaskMember `gorm:"foreignKey:TaskID" json:"members,omitempty"`
}

func (ApprovalTask) TableName() string { return "sys_approval_task" }

// ApprovalTaskMember corresponds to sys_approval_task_member: one approver's
// record within a task.
type ApprovalTaskMember struct {
	BaseModel
	TaskID     uint       `gorm:"column:task_id;not null;index" json:"task_id"`
	UserID     uint       `gorm:"column:user_id;not null;index" json:"user_id"`
	UserName   string     `gorm:"column:user_name;size:64" json:"user_name"`
	Status     string     `gorm:"column:status;size:16;not null;default:PENDING" json:"status"`
	Comment    string     `gorm:"column:comment;size:255" json:"comment"`
	ApprovedAt *time.Time `gorm:"column:approved_at" json:"approved_at"`
}

func (ApprovalTaskMember) TableName() string { return "sys_approval_task_member" }
