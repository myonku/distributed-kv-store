package raft_store

import (
	"context"
	"distributed-kv-store/internal/storage"
)

// StateMachine 抽象：Raft 日志 commit 后由 Node 调用
type StateMachine interface {
	Apply(index uint64, cmd storage.Command) error
}

// 基于 Storage 的简单 KV 状态机实现。
// 在 Raft 模式下：
//   - Raft 自己维护一份 raft log；
//   - 这里再在 Storage 上维护一份业务 WAL（AppendLog + ApplyLog），用于重放和查询。
type KVStateMachine struct {
	St storage.Storage
}

// 按 Raft 提交顺序将命令追加到底层 WAL，并应用到状态机
func (sm *KVStateMachine) Apply(index uint64, cmd storage.Command) error {
	idx, err := sm.St.AppendLog(context.TODO(), cmd)
	if err != nil {
		return err
	}
	if idx != index {
		// 理论上不应该发生
		return nil
	}
	return sm.St.ApplyLog(context.TODO(), idx)
}
