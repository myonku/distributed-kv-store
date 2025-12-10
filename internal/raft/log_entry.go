package raft

import (
	"time"

	"distributed-kv-store/internal/store"
)

// 日志条目
type LogEntry struct {
	Index uint64
	Term  uint64
	Cmd   store.Command
}

// 内部 goroutine：把 commitIndex 之前的日志逐条 Apply 到状态机
func (n *Node) runApplyLoop() {
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
		}

		var entry LogEntry
		var ok bool

		n.mu.Lock()
		if n.commitIndex > n.lastApplied && int(n.lastApplied) < len(n.log) {
			// 日志索引从 1 开始，slice 下标从 0 开始
			idx := n.lastApplied + 1
			entry = n.log[idx-1]
			n.lastApplied = idx
			ok = true
		}
		n.mu.Unlock()

		if !ok {
			// 没有可应用的日志，稍作休眠避免 busy loop
			time.Sleep(10 * time.Millisecond)
			continue
		}

		n.applyEntry(entry)
	}
}

// 内部调用：实际执行 Apply
func (n *Node) applyEntry(entry LogEntry) {
	if n.sm != nil {
		// 这里忽略返回错误，只在 ApplyResult 中透出
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
