package repository

import (
	"context"

	"gorm.io/gorm"

	"scm/internal/model"
)

type LogisticsRepo struct{ *genericRepo[model.LogisticsQuery] }

func NewLogisticsRepo(db *gormDB) *LogisticsRepo {
	return &LogisticsRepo{genericRepo: newGenericRepo[model.LogisticsQuery](db)}
}

func (r *LogisticsRepo) FindByTrackingNo(ctx context.Context, t, userID uint, trackingNo string) (*model.LogisticsQuery, error) {
	var q model.LogisticsQuery
	if err := r.db.DB.Where("tenant_id = ? AND user_id = ? AND tracking_no = ?", t, userID, trackingNo).
		Order("id DESC").First(&q).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &q, nil
}

func (r *LogisticsRepo) List(ctx context.Context, t, userID uint, page, size int) ([]model.LogisticsQuery, int64, error) {
	var total int64
	var out []model.LogisticsQuery
	if err := r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).
		Order("id DESC").Offset((page-1)*size).Limit(size).Find(&out).Offset(-1).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	return out, total, nil
}