package repository

import "gorm.io/gorm"

// errorsIsNotFound normalizes GORM's not-found sentinel.
func errorsIsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
