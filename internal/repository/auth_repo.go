package repository

import (
	"gorm.io/gorm"

	"scm/internal/model"
)

// UserRepo owns sys_user.
type UserRepo struct{ *genericRepo[model.User] }

func NewUserRepo(db *gormDB) *UserRepo {
	return &UserRepo{genericRepo: newGenericRepo[model.User](db)}
}

// GetByUsername looks a user up by login name; nil when absent.
func (r *UserRepo) GetByUsername(username string) (*model.User, error) {
	var u model.User
	if err := r.db.DB.Where("username = ?", username).First(&u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}
