package model

import "time"

// ApiKey corresponds to sys_api_key: an AK/SK credential for agent / MCP
// access. The SK is returned in plaintext only at issue time; at rest it is
// stored as an AES-GCM ciphertext (SKCipher) so signatures remain verifiable.
type ApiKey struct {
	BaseModel
	TenantID    uint       `gorm:"column:tenant_id;not null;index" json:"tenant_id"`
	UserID      uint       `gorm:"column:user_id;not null;index" json:"user_id"`
	Name        string     `gorm:"column:name;size:64;not null" json:"name"`
	AK          string     `gorm:"column:ak;size:64;uniqueIndex;not null" json:"ak"`
	SKCipher    string     `gorm:"column:sk_cipher;size:256;not null" json:"-"`
	Permissions string     `gorm:"column:permissions;size:1024" json:"permissions"` // comma-joined perm codes; empty = all
	Status      int8       `gorm:"column:status;default:1" json:"status"`
	ExpiresAt   *time.Time `gorm:"column:expires_at" json:"expires_at"`
}

func (ApiKey) TableName() string { return "sys_api_key" }
