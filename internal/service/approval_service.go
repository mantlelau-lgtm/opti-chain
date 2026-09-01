package service

import (
	"time"

	"gorm.io/gorm"

	"scm/internal/model"
	"scm/internal/pkg/authx"
	"scm/internal/repository"
)

// ApprovalService owns the approval workflow: groups, submitting orders for
// approval, and the workbench (pending/processed/submitted) with 会签/或签.
type ApprovalService struct {
	groups  *repository.ApprovalGroupRepo
	tasks   *repository.ApprovalTaskRepo
	users   *repository.UserRepo
	poSvc   *PurchaseOrderService
	soSvc   *SalesOrderService
	db      *gorm.DB
}

// ApprovalDeps groups the dependencies an ApprovalService needs.
type ApprovalDeps struct {
	Groups *repository.ApprovalGroupRepo
	Tasks  *repository.ApprovalTaskRepo
	Users  *repository.UserRepo
	POSvc  *PurchaseOrderService
	SOSvc  *SalesOrderService
	DB     *gorm.DB
}

func NewApprovalService(d ApprovalDeps) *ApprovalService {
	return &ApprovalService{
		groups: d.Groups, tasks: d.Tasks, users: d.Users,
		poSvc: d.POSvc, soSvc: d.SOSvc, db: d.DB,
	}
}

// ---- group management ----

func (s *ApprovalService) ListGroups(t uint, in PageInput) ([]model.ApprovalGroup, int64, error) {
	var (
		out   []model.ApprovalGroup
		total int64
	)
	f := repository.ListFilter{Page: in.Page, Keyword: in.Keyword, Tenant: t}
	if err := s.groups.List(t, f, &out, &total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *ApprovalService) CreateGroup(t uint, g *model.ApprovalGroup) (*model.ApprovalGroup, error) {
	if g.Name == "" || g.OrderType == "" || len(g.Members) == 0 {
		return nil, errorsBadRequest("name, order_type and at least one member are required")
	}
	if g.OrderType != model.ApprovalTypePO && g.OrderType != model.ApprovalTypeSO {
		return nil, errorsBadRequest("order_type must be PO or SO")
	}
	if g.Mode != model.ApprovalModeAll && g.Mode != model.ApprovalModeAny {
		g.Mode = model.ApprovalModeAll
	}
	// one group per order type: reject duplicates instead of shadowing.
	if existing, _ := s.groups.ListByType(t, g.OrderType); len(existing) > 0 {
		return nil, errf(ErrConflict, "该订单类型已配置审批组，请编辑现有审批组")
	}
	if err := s.groups.CreateWithMembers(t, g); err != nil {
		return nil, err
	}
	return s.groups.GetWithMembers(t, g.ID)
}

func (s *ApprovalService) UpdateGroup(t, id uint, g *model.ApprovalGroup) (*model.ApprovalGroup, error) {
	if g.Name == "" || len(g.Members) == 0 {
		return nil, errorsBadRequest("name and at least one member are required")
	}
	if g.Mode != model.ApprovalModeAll && g.Mode != model.ApprovalModeAny {
		g.Mode = model.ApprovalModeAll
	}
	// preserve audit fields (the bound struct has zero CreatedAt/CreatedBy).
	if old, _ := s.groups.Get(t, id); old != nil {
		g.CreatedAt = old.CreatedAt
		g.CreatedBy = old.CreatedBy
	}
	g.ID = id
	if err := s.groups.UpdateWithMembers(t, g); err != nil {
		return nil, err
	}
	return s.groups.GetWithMembers(t, id)
}

func (s *ApprovalService) DeleteGroup(t, id uint) error {
	return s.groups.Delete(t, id)
}

// ---- submit / workbench ----

// Submit creates an approval task for an order using its order-type group.
func (s *ApprovalService) Submit(t uint, a *authx.Actor, orderType string, orderID uint) (*model.ApprovalTask, error) {
	if a == nil {
		return nil, errorsBadRequest("authenticated user required")
	}
	if orderType != model.ApprovalTypePO && orderType != model.ApprovalTypeSO {
		return nil, errorsBadRequest("order_type must be PO or SO")
	}

	orderNumber, err := s.resolveOrder(t, orderType, orderID)
	if err != nil {
		return nil, err
	}
	groups, err := s.groups.ListByType(t, orderType)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, errorsBadRequest("该订单类型未配置审批组")
	}
	group := groups[0] // one group per order type (v1)
	if len(group.Members) == 0 {
		return nil, errorsBadRequest("审批组没有成员")
	}

	task := &model.ApprovalTask{
		OrderType:    orderType,
		OrderID:      orderID,
		OrderNumber:  orderNumber,
		Status:       model.ApprovalStatusPending,
		SubmitterID:  a.UserID,
		SubmitterName: a.Username,
	}
	for _, m := range group.Members {
		name := ""
		if u, _ := s.users.Get(t, m.UserID); u != nil {
			name = u.Name
			if name == "" {
				name = u.Username
			}
		}
		task.Members = append(task.Members, model.ApprovalTaskMember{
			UserID:   m.UserID,
			UserName: name,
			Status:   model.ApprovalStatusPending,
		})
	}
	if err := s.tasks.CreateWithMembers(t, task); err != nil {
		return nil, err
	}
	return s.tasks.GetWithMembers(t, task.ID)
}

func (s *ApprovalService) resolveOrder(t uint, orderType string, orderID uint) (string, error) {
	switch orderType {
	case model.ApprovalTypePO:
		po, err := s.poSvc.Get(t, orderID)
		if po == nil || err != nil {
			return "", errNotFound(orderID)
		}
		return po.PONumber, nil
	case model.ApprovalTypeSO:
		so, err := s.soSvc.Get(t, orderID)
		if so == nil || err != nil {
			return "", errNotFound(orderID)
		}
		return so.SONumber, nil
	}
	return "", errorsBadRequest("unknown order type")
}

func (s *ApprovalService) GetTask(t, id uint) (*model.ApprovalTask, error) {
	return s.tasks.GetWithMembers(t, id)
}

func (s *ApprovalService) Pending(t, userID uint) ([]model.ApprovalTask, error) {
	var out []model.ApprovalTask
	if err := s.tasks.PendingForUser(t, userID, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ApprovalService) Processed(t, userID uint) ([]model.ApprovalTask, error) {
	var out []model.ApprovalTask
	if err := s.tasks.ProcessedForUser(t, userID, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ApprovalService) Submitted(t, userID uint) ([]model.ApprovalTask, error) {
	var out []model.ApprovalTask
	if err := s.tasks.SubmittedBy(t, userID, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Act records the current user's approval/rejection and re-evaluates the task.
// On full approval it triggers the order's own approval side effect.
func (s *ApprovalService) Act(t uint, taskID, userID uint, action, comment string) (*model.ApprovalTask, error) {
	task, err := s.tasks.GetWithMembers(t, taskID)
	if task == nil {
		return nil, errNotFound(taskID)
	}
	if err != nil {
		return nil, err
	}
	if task.Status != model.ApprovalStatusPending {
		return nil, errorsBadRequest("该审批任务已结束")
	}

	var me *model.ApprovalTaskMember
	for i := range task.Members {
		if task.Members[i].UserID == userID {
			me = &task.Members[i]
			break
		}
	}
	if me == nil {
		return nil, errf(ErrForbidden, "你不是该审批任务的审批人")
	}
	if me.Status != model.ApprovalStatusPending {
		return nil, errorsBadRequest("你已处理过该审批")
	}
	if action != model.ApprovalStatusApproved && action != model.ApprovalStatusRejected {
		return nil, errorsBadRequest("action must be APPROVE or REJECT")
	}

	now := time.Now()
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.ApprovalTaskMember{}).
			Where("id = ?", me.ID).
			Updates(map[string]any{"status": action, "comment": comment, "approved_at": now}).Error; err != nil {
			return err
		}
		// re-evaluate the whole task ON THE SAME tx so the uncommitted member
		// update is visible.
		final, err := s.evaluateTx(tx, t, taskID)
		if err != nil {
			return err
		}
		if final != model.ApprovalStatusPending {
			if err := tx.Model(&model.ApprovalTask{}).
				Where("id = ?", taskID).
				Update("status", final).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// trigger order side effects on completion.
	updated, _ := s.tasks.GetWithMembers(t, taskID)
	if updated != nil && updated.Status == model.ApprovalStatusApproved {
		if err := s.applyOrderApproval(t, updated); err != nil {
			return nil, err
		}
	}
	return s.tasks.GetWithMembers(t, taskID)
}

// evaluateTx determines the task's final status, reading member records from
// the caller's transaction.
func (s *ApprovalService) evaluateTx(tx *gorm.DB, t, taskID uint) (string, error) {
	var task model.ApprovalTask
	if err := tx.Preload("Members").Where("tenant_id = ?", t).First(&task, taskID).Error; err != nil {
		return "", err
	}
	all := len(task.Members) > 0
	anyApproved := false
	for _, m := range task.Members {
		if m.Status == model.ApprovalStatusRejected {
			return model.ApprovalStatusRejected, nil
		}
		if m.Status == model.ApprovalStatusApproved {
			anyApproved = true
		} else {
			all = false
		}
	}
	// mode comes from the group; look it up by order type.
	groups, _ := s.groups.ListByType(t, task.OrderType)
	mode := model.ApprovalModeAll
	if len(groups) > 0 {
		mode = groups[0].Mode
	}
	if (mode == model.ApprovalModeAll && all) || (mode == model.ApprovalModeAny && anyApproved) {
		return model.ApprovalStatusApproved, nil
	}
	return model.ApprovalStatusPending, nil
}

// applyOrderApproval performs the order's own approval side effect.
func (s *ApprovalService) applyOrderApproval(t uint, task *model.ApprovalTask) error {
	switch task.OrderType {
	case model.ApprovalTypePO:
		return s.poSvc.SetStatus(t, task.OrderID, model.POStatusApproved)
	case model.ApprovalTypeSO:
		_, err := s.soSvc.Approve(t, task.OrderID)
		return err
	}
	return nil
}
