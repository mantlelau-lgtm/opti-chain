package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"scm/internal/model"
)

// ApprovalGroupRepo owns approval groups + members.
type ApprovalGroupRepo struct {
	*tenantRepo[model.ApprovalGroup]
	db *gormDB
}

func NewApprovalGroupRepo(db *gormDB) *ApprovalGroupRepo {
	return &ApprovalGroupRepo{tenantRepo: newTenantRepo[model.ApprovalGroup](db), db: db}
}

func (r *ApprovalGroupRepo) GetWithMembers(t, id uint) (*model.ApprovalGroup, error) {
	var g model.ApprovalGroup
	if err := r.db.DB.Preload("Members").Where("tenant_id = ?", t).First(&g, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

// ListByType returns groups for an order type, latest first.
func (r *ApprovalGroupRepo) ListByType(t uint, orderType string) ([]model.ApprovalGroup, error) {
	var out []model.ApprovalGroup
	err := r.db.DB.Preload("Members").
		Where("tenant_id = ? AND order_type = ?", t, orderType).
		Order("id DESC").
		Find(&out).Error
	return out, err
}

// CreateWithMembers persists a group and its members in one transaction.
func (r *ApprovalGroupRepo) CreateWithMembers(t uint, g *model.ApprovalGroup) error {
	g.TenantID = t
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(g).Error; err != nil {
			return err
		}
		for i := range g.Members {
			g.Members[i].GroupID = g.ID
			if err := tx.Create(&g.Members[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateWithMembers replaces a group's members.
func (r *ApprovalGroupRepo) UpdateWithMembers(t uint, g *model.ApprovalGroup) error {
	g.TenantID = t
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ?", g.ID).Delete(&model.ApprovalGroupMember{}).Error; err != nil {
			return err
		}
		for i := range g.Members {
			g.Members[i].ID = 0
			g.Members[i].GroupID = g.ID
			if err := tx.Create(&g.Members[i]).Error; err != nil {
				return err
			}
		}
		saved := g.Members
		g.Members = nil
		err := tx.Where("tenant_id = ?", t).Save(g).Error
		g.Members = saved
		return err
	})
}

func (r *ApprovalGroupRepo) List(t uint, f ListFilter, out *[]model.ApprovalGroup, total *int64) error {
	return r.listT(f, func(q *gorm.DB) *gorm.DB { return q.Preload("Members").Order("id") }, out, total)
}

// ApprovalTaskRepo owns approval tasks + member records.
type ApprovalTaskRepo struct {
	*tenantRepo[model.ApprovalTask]
	db *gormDB
}

func NewApprovalTaskRepo(db *gormDB) *ApprovalTaskRepo {
	return &ApprovalTaskRepo{tenantRepo: newTenantRepo[model.ApprovalTask](db), db: db}
}

func (r *ApprovalTaskRepo) GetWithMembers(t, id uint) (*model.ApprovalTask, error) {
	var task model.ApprovalTask
	if err := r.db.DB.Preload("Members").Where("tenant_id = ?", t).First(&task, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// CreateWithMembers persists a task and its member records in one transaction.
func (r *ApprovalTaskRepo) CreateWithMembers(t uint, task *model.ApprovalTask) error {
	task.TenantID = t
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(task).Error; err != nil {
			return err
		}
		for i := range task.Members {
			task.Members[i].TaskID = task.ID
			if err := tx.Create(&task.Members[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// PendingForUser lists tasks where the user has a PENDING member record.
func (r *ApprovalTaskRepo) PendingForUser(t, userID uint, out *[]model.ApprovalTask) error {
	return r.db.DB.Preload("Members").
		Where("tenant_id = ? AND id IN (?)",
			t,
			r.db.DB.Model(&model.ApprovalTaskMember{}).Select("task_id").
				Where("user_id = ? AND status = ?", userID, model.ApprovalStatusPending),
		).
		Order("id DESC").Find(out).Error
}

// ProcessedForUser lists tasks the user has acted on (approved/rejected).
func (r *ApprovalTaskRepo) ProcessedForUser(t, userID uint, out *[]model.ApprovalTask) error {
	return r.db.DB.Preload("Members").
		Where("tenant_id = ? AND id IN (?)",
			t,
			r.db.DB.Model(&model.ApprovalTaskMember{}).Select("task_id").
				Where("user_id = ? AND status <> ?", userID, model.ApprovalStatusPending),
		).
		Order("id DESC").Find(out).Error
}

// SubmittedBy lists tasks submitted by a user.
func (r *ApprovalTaskRepo) SubmittedBy(t, userID uint, out *[]model.ApprovalTask) error {
	return r.db.DB.Preload("Members").
		Where("tenant_id = ? AND submitter_id = ?", t, userID).
		Order("id DESC").Find(out).Error
}
