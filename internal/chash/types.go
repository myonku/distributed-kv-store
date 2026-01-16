package chash

// MigrationStatus 表示迁移计划项当前状态
type MigrationStatus int

const (
	MigrationStatusPlanned MigrationStatus = iota
	MigrationStatusInProgress
	MigrationStatusCompleted
)

// 用于数据再平衡的计划结构体
type MovePlan struct {
	Epoch    uint64         // 计划所属的 epoch
	Moves    []MoveRange    // 节点ID->数据迁移范围映射
	CopyOnly bool           // 是否仅执行数据复制（不删除旧数据）
	Hints    []MovePlanHint // 迁移提示（旧/新 owner 集合）
}

// 一轮数据迁移操作
type MoveRange struct {
	FromID    string // 源节点 ID
	ToNodeID  string // 目标节点 ID
	StartHash uint32 // 起始哈希值（包含）
	EndHash   uint32 // 结束哈希值（不包含）
}

// MovePlanHint 描述某个 hash 范围在迁移窗口内的旧/新 owner 集合以及状态
type MovePlanHint struct {
	Epoch     uint64          // 该计划所属的 ring 版本
	StartHash uint32          // 起始哈希（含）
	EndHash   uint32          // 结束哈希（不含）
	OldOwners []string        // 在切换前负责该范围的节点 ID 列表
	NewOwners []string        // 切换后负责该范围的节点 ID 列表
	Status    MigrationStatus // 该范围的迁移状态
}
