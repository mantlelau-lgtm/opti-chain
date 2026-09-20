package model

import "time"

// Conversation turns are no longer stored in the database: they live in
// JSONL files under the configured memory data dir (see internal/memory).
// The tables below back the long-term knowledge graph only.
//
// The legacy sys_assistant_memory table is drained by cmd/migrate-memory.

// MemoryNode is an entity node in the user's personal knowledge graph.
// NodeType identifies the kind (e.g. MATERIAL, SUPPLIER); EntityID links
// to the business table when the node corresponds to a real record.
type MemoryNode struct {
	BaseModel
	TenantID uint   `gorm:"column:tenant_id;not null;index"`
	UserID   uint   `gorm:"column:user_id;not null;index"`
	NodeType string `gorm:"column:node_type;size:32;not null"`
	EntityID uint   `gorm:"column:entity_id;default:0"`
	Label    string `gorm:"column:label;size:128;not null"`
	Metadata string `gorm:"column:metadata;size:1024"` // JSON
}

func (MemoryNode) TableName() string { return "sys_memory_node" }

// MemoryEdge is a directed relationship between two nodes. Weight decays
// over time so infrequent edges naturally fade.
type MemoryEdge struct {
	BaseModel
	TenantID     uint      `gorm:"column:tenant_id;not null;index"`
	UserID       uint      `gorm:"column:user_id;not null;index"`
	FromNodeID   uint      `gorm:"column:from_node_id;not null;index"`
	ToNodeID     uint      `gorm:"column:to_node_id;not null;index"`
	RelationType string    `gorm:"column:relation_type;size:32;not null"`
	Weight       float64   `gorm:"column:weight;default:1"`
	LastUpdated  time.Time `gorm:"column:last_updated"`
}

func (MemoryEdge) TableName() string { return "sys_memory_edge" }

// MemoryProfile stores a compact user profile as JSON, updated by the
// consolidation step. One row per (tenant, user).
type MemoryProfile struct {
	BaseModel
	TenantID    uint   `gorm:"column:tenant_id;not null;uniqueIndex:idx_mem_prof"`
	UserID      uint   `gorm:"column:user_id;not null;uniqueIndex:idx_mem_prof"`
	ProfileJSON string `gorm:"column:profile_json;type:text"`
}

func (MemoryProfile) TableName() string { return "sys_memory_profile" }
