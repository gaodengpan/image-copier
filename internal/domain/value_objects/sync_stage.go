package value_objects

// SyncStage 定义 sync 命令的阶段
type SyncStage int

const (
	SyncStageChecking SyncStage = iota // 检查镜像存在性
	SyncStageSyncing                   // 同步到中转仓库
	SyncStageDistributing              // 分发到目标
)

// String 返回阶段的显示名称
func (s SyncStage) String() string {
	switch s {
	case SyncStageChecking:
		return "checking"
	case SyncStageSyncing:
		return "sync"
	case SyncStageDistributing:
		return "dist"
	default:
		return "unknown"
	}
}