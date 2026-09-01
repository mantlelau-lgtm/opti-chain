package repository

import (
	"gorm.io/gorm"

	"scm/internal/model"
)

// ApiKeyRepo owns sys_api_key. Keys are tenant-scoped; the AK is globally
// unique (unique index) because signatures resolve tenant from the AK alone.
type ApiKeyRepo struct{ *genericRepo[model.ApiKey] }

func NewApiKeyRepo(db *gormDB) *ApiKeyRepo {
	return &ApiKeyRepo{genericRepo: newGenericRepo[model.ApiKey](db)}
}

// GetByAK resolves a key by its access key (globally unique); nil when absent.
func (r *ApiKeyRepo) GetByAK(ak string) (*model.ApiKey, error) {
	var k model.ApiKey
	if err := r.db.DB.Where("ak = ?", ak).First(&k).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &k, nil
}

// Get returns one key scoped to the tenant AND its owner user, nil when absent.
func (r *ApiKeyRepo) Get(t, userID, id uint) (*model.ApiKey, error) {
	var k model.ApiKey
	if err := r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).First(&k, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &k, nil
}

// Create inserts a key (tenant/user already stamped on the model by the caller).
func (r *ApiKeyRepo) Create(k *model.ApiKey) error {
	return r.db.DB.Create(k).Error
}

// Update persists a key within the tenant.
func (r *ApiKeyRepo) Update(t uint, k *model.ApiKey) error {
	k.TenantID = t
	return r.db.DB.Save(k).Error
}

// Delete removes a key owned by the given user within the tenant.
func (r *ApiKeyRepo) Delete(t, userID, id uint) error {
	return r.db.DB.Where("tenant_id = ? AND user_id = ?", t, userID).Delete(&model.ApiKey{}, id).Error
}

// List returns paginated keys owned by the given user.
func (r *ApiKeyRepo) List(t, userID uint, f ListFilter, out *[]model.ApiKey, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		q = q.Where("tenant_id = ? AND user_id = ?", t, userID)
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("name LIKE ? OR ak LIKE ?", like, like)
		}
		return q.Order("id DESC")
	}
	return paginate(r.db.DB, f, apply, out, total)
}
