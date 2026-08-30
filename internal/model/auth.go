package model

// User corresponds to sys_user. PasswordHash never leaves the backend
// (json:"-").
type User struct {
	BaseModel
	Username     string `gorm:"column:username;size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string `gorm:"column:password_hash;size:128;not null" json:"-"`
	Name         string `gorm:"column:name;size:64" json:"name"`
	Status       int8   `gorm:"column:status;default:1" json:"status"`
}

func (User) TableName() string { return "sys_user" }
