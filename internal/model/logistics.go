package model

import "time"

// LogisticsQuery stores a single tracking-query result. The routes field is
// the raw JSON response from SF's routeResps.
type LogisticsQuery struct {
	BaseModel
	TenantID     uint   `gorm:"column:tenant_id;not null;index"`
	UserID       uint   `gorm:"column:user_id;not null;index"`
	TrackingNo   string `gorm:"column:tracking_no;size:64;not null;index"`
	LatestStatus string `gorm:"column:latest_status;size:128"`
	Routes       string `gorm:"column:routes;type:text"` // JSON array of route nodes
	QueryAt      time.Time `gorm:"column:query_at"`
}

func (LogisticsQuery) TableName() string { return "sys_logistics_query" }