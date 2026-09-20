package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"scm/internal/model"
)

// ---------------------------------------------------------------------------
// MemoryNode
// ---------------------------------------------------------------------------

type MemoryNodeRepo struct{ *genericRepo[model.MemoryNode] }

func NewMemoryNodeRepo(db *gormDB) *MemoryNodeRepo {
	return &MemoryNodeRepo{genericRepo: newGenericRepo[model.MemoryNode](db)}
}

func (r *MemoryNodeRepo) FindOrCreate(ctx context.Context, t, userID uint, nodeType string, entityID uint, label string) (*model.MemoryNode, error) {
	var n model.MemoryNode
	err := r.db.DB.Where("tenant_id = ? AND user_id = ? AND node_type = ? AND entity_id = ?", t, userID, nodeType, entityID).
		First(&n).Error
	if err == nil {
		return &n, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	n = model.MemoryNode{TenantID: t, UserID: userID, NodeType: nodeType, EntityID: entityID, Label: label}
	if err := r.db.DB.Create(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

func (r *MemoryNodeRepo) FindByLabel(ctx context.Context, t, userID uint, label string) (*model.MemoryNode, error) {
	var n model.MemoryNode
	if err := r.db.DB.Where("tenant_id = ? AND user_id = ? AND label = ?", t, userID, label).
		First(&n).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &n, nil
}

func (r *MemoryNodeRepo) ListByUser(ctx context.Context, t, userID uint) ([]model.MemoryNode, error) {
	var out []model.MemoryNode
	if err := r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *MemoryNodeRepo) DeleteByUser(ctx context.Context, t, userID uint) error {
	return r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).Delete(&model.MemoryNode{}).Error
}

// ---------------------------------------------------------------------------
// MemoryEdge
// ---------------------------------------------------------------------------

type MemoryEdgeRepo struct{ *genericRepo[model.MemoryEdge] }

func NewMemoryEdgeRepo(db *gormDB) *MemoryEdgeRepo {
	return &MemoryEdgeRepo{genericRepo: newGenericRepo[model.MemoryEdge](db)}
}

func (r *MemoryEdgeRepo) Upsert(ctx context.Context, t, userID uint, fromID, toID uint, relationType string, weight float64) error {
	var e model.MemoryEdge
	err := r.db.DB.Where("tenant_id = ? AND user_id = ? AND from_node_id = ? AND to_node_id = ? AND relation_type = ?",
		t, userID, fromID, toID, relationType).First(&e).Error
	if err == nil {
		e.Weight += weight
		e.LastUpdated = time.Now()
		return r.db.DB.Save(&e).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	e = model.MemoryEdge{
		TenantID: t, UserID: userID, FromNodeID: fromID, ToNodeID: toID,
		RelationType: relationType, Weight: weight, LastUpdated: time.Now(),
	}
	return r.db.DB.Create(&e).Error
}

func (r *MemoryEdgeRepo) TopEdges(ctx context.Context, t, userID uint, limit int) ([]model.MemoryEdge, error) {
	var out []model.MemoryEdge
	if err := r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).
		Order("weight DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *MemoryEdgeRepo) DeleteByUser(ctx context.Context, t, userID uint) error {
	return r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).Delete(&model.MemoryEdge{}).Error
}

// ApplyDecay is called periodically by the memory decay loop.
//   - olderThanDays: only touch edges that haven't been updated in this many days
//   - factor: multiply weight by this (typically <1, e.g. 0.9 for -10%)
//   - floor:  edges with a resulting weight strictly below this are DELETED
//
// Returns (decayedCount, purgedCount, error). The UPDATE + DELETE are wrapped
// in a single transaction so the two counters stay consistent.
func (r *MemoryEdgeRepo) ApplyDecay(ctx context.Context, olderThanDays int, factor, floor float64) (int64, int64, error) {
	if olderThanDays <= 0 {
		olderThanDays = 7
	}
	if factor <= 0 || factor >= 1 {
		factor = 0.9
	}
	if floor < 0 {
		floor = 0
	}
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	var decayed, purged int64
	err := r.db.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.MemoryEdge{}).
			Where("last_updated <= ?", cutoff).
			Update("weight", gorm.Expr("weight * ?", factor))
		if res.Error != nil {
			return res.Error
		}
		decayed = res.RowsAffected
		res = tx.Where("weight < ? AND last_updated <= ?", floor, cutoff).
			Delete(&model.MemoryEdge{})
		if res.Error != nil {
			return res.Error
		}
		purged = res.RowsAffected
		return nil
	})
	return decayed, purged, err
}

// ---------------------------------------------------------------------------
// MemoryProfile
// ---------------------------------------------------------------------------

type MemoryProfileRepo struct {
	*genericRepo[model.MemoryProfile]
}

func NewMemoryProfileRepo(db *gormDB) *MemoryProfileRepo {
	return &MemoryProfileRepo{genericRepo: newGenericRepo[model.MemoryProfile](db)}
}

func (r *MemoryProfileRepo) Upsert(ctx context.Context, t, userID uint, profileJSON string) error {
	var p model.MemoryProfile
	err := r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).First(&p).Error
	if err == nil {
		p.ProfileJSON = profileJSON
		return r.db.DB.Save(&p).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	p = model.MemoryProfile{TenantID: t, UserID: userID, ProfileJSON: profileJSON}
	return r.db.DB.Create(&p).Error
}

func (r *MemoryProfileRepo) Get(ctx context.Context, t, userID uint) (*model.MemoryProfile, error) {
	var p model.MemoryProfile
	if err := r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).First(&p).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *MemoryProfileRepo) DeleteByUser(ctx context.Context, t, userID uint) error {
	return r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).Delete(&model.MemoryProfile{}).Error
}
