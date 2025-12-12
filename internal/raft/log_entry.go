package raft

import (
	"time"

	"distributed-kv-store/internal/raft/raft_store"
)

// 内部 goroutine：把 commitIndex 之前的日志逐条 Apply 到状态机
func (n *Node) runApplyLoop() {
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
		}

		var entry raft_store.LogEntry
		var ok bool
		var idx uint64

		n.mu.Lock()
		if n.commitIndex > n.lastApplied {
			idx = n.lastApplied + 1 // 日志索引从 1 开始
			n.lastApplied = idx
			ok = true
		}
		n.mu.Unlock()

		if !ok {
			// 没有可应用的日志，稍作休眠避免 busy loop
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if n.logStore == nil {
			// 没有日志存储，稍作休眠
			time.Sleep(10 * time.Millisecond)
			continue
		}

		entry, err := n.logStore.Entry(idx)
		if err != nil || entry.Index == 0 {
			// 读取失败或日志不存在，稍作休眠重试
			time.Sleep(10 * time.Millisecond)
			continue
		}

		n.applyEntry(entry)
	}
}

// 内部调用：实际执行 Apply
func (n *Node) applyEntry(entry raft_store.LogEntry) {
	if n.sm != nil {
		if err := n.sm.Apply(entry.Index, entry.Cmd); err != nil {
			select {
			case n.applyCh <- ApplyResult{Index: entry.Index, Term: entry.Term, Err: err}:
			default:
			}
			return
		}
	}

	// 将应用结果通知给等待方（例如 Propose 调用者）
	select {
	case n.applyCh <- ApplyResult{Index: entry.Index, Term: entry.Term, Err: nil}:
	default:
	}
}
