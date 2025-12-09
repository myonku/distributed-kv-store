package store

import "context"

// 定义应用于状态机的接口
type StateMachine interface {
	Apply(index uint64, cmd Command) error
}

// 状态机接口：Raft 日志 commit 后调用
type KVStateMachine struct {
	St Storage
}

// Apply 应用日志到状态机
func (sm *KVStateMachine) Apply(index uint64, cmd Command) error {
	// 这里可以直接走 Storage 的接口
	idx, err := sm.St.AppendLog(context.TODO(), cmd)
	if err != nil {
		return err
	}
	if idx != index {
		// 对齐逻辑，早期可以不管
	}
	return sm.St.ApplyLog(context.TODO(), idx)
}
