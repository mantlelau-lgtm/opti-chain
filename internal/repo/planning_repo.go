package repository

import (
	"gorm.io/gorm"

	"scm/internal/model"
)

// DemandRepo owns plan_demand.
type DemandRepo struct{ *tenantRepo[model.Demand] }

func NewDemandRepo(db *gormDB) *DemandRepo {
	return &DemandRepo{tenantRepo: newTenantRepo[model.Demand](db)}
}

func (r *DemandRepo) List(f ListFilter, out *[]model.Demand, total *int64) error {
	return r.listT(f, func(q *gorm.DB) *gorm.DB { return q.Order("id DESC") }, out, total)
}

// SumOpenByMaterial returns total open demand qty per material (map mat->qty)
// within one tenant.
func (r *DemandRepo) SumOpenByMaterial(t uint) (map[uint]float64, error) {
	type row struct {
		MaterialID uint    `gorm:"column:material_id"`
		Qty        float64 `gorm:"column:qty"`
	}
	var rows []row
	err := r.db.DB.Model(&model.Demand{}).
		Where("tenant_id = ? AND status = ?", t, model.DemandStatusOpen).
		Select("material_id, COALESCE(SUM(demand_qty),0) AS qty").
		Group("material_id").Scan(&rows).Error
	m := make(map[uint]float64, len(rows))
	for _, r := range rows {
		m[r.MaterialID] = r.Qty
	}
	return m, err
}

// MrpResultRepo owns plan_mrp_result.
type MrpResultRepo struct{ *tenantRepo[model.MrpResult] }

func NewMrpResultRepo(db *gormDB) *MrpResultRepo {
	return &MrpResultRepo{tenantRepo: newTenantRepo[model.MrpResult](db)}
}

func (r *MrpResultRepo) List(f ListFilter, out *[]model.MrpResult, total *int64) error {
	return r.listT(f, func(q *gorm.DB) *gorm.DB { return q.Order("id DESC") }, out, total)
}

// BatchCreate stamps the tenant and inserts a computation batch.
func (r *MrpResultRepo) BatchCreate(t uint, list []model.MrpResult) error {
	for i := range list {
		list[i].TenantID = t
	}
	if len(list) == 0 {
		return nil
	}
	return r.db.DB.Create(&list).Error
}
