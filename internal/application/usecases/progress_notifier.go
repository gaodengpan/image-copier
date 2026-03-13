package use_cases

import (
	"github.com/gaodengpan/image-copier/internal/domain/ports/input"
	"github.com/gaodengpan/image-copier/internal/domain/value_objects"
)

// ProgressNotifier 封装进度通知逻辑，消除重复代码
type ProgressNotifier struct {
	callback input.SyncStageCallback
}

// NewProgressNotifier 创建进度通知器
func NewProgressNotifier(callback input.SyncStageCallback) *ProgressNotifier {
	return &ProgressNotifier{callback: callback}
}

// Notify 安全地发送进度通知（nil-safe）
func (n *ProgressNotifier) Notify(imageID string, stage value_objects.SyncStage, targetName string, percent float64) {
	if n.callback != nil {
		n.callback(imageID, stage, targetName, percent)
	}
}

// NotifyChecking 通知检查阶段进度
func (n *ProgressNotifier) NotifyChecking(imageID string, percent float64) {
	n.Notify(imageID, value_objects.SyncStageChecking, "", percent)
}

// NotifySyncing 通知同步阶段进度
func (n *ProgressNotifier) NotifySyncing(imageID string, percent float64) {
	n.Notify(imageID, value_objects.SyncStageSyncing, "", percent)
}

// NotifyDistributing 通知分发阶段进度
func (n *ProgressNotifier) NotifyDistributing(imageID string, targetName string, percent float64) {
	n.Notify(imageID, value_objects.SyncStageDistributing, targetName, percent)
}

// 进度常量，避免魔法数字
const (
	ProgressStart   = 0.0
	ProgressHalfway = 50.0
	ProgressDone    = 100.0
)
