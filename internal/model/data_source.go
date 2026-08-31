package model

// DataSource corresponds to sys_data_source: a target database connection for
// the platform-level storage migration feature. Stored DSNs include credentials
// and are therefore platform-only data.
type DataSource struct {
	BaseModel
	Name   string `gorm:"column:name;size:64;not null" json:"name"`
	Driver string `gorm:"column:driver;size:16;not null" json:"driver"` // mysql | postgres
	DSN    string `gorm:"column:dsn;size:512;not null" json:"dsn"`
}

func (DataSource) TableName() string { return "sys_data_source" }
