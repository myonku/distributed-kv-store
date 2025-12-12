package storage

import (
	"context"
	"sync"

	"distributed-kv-store/configs"
)

// 内存实现 + 简单的 index 递增，用于单机/早期开发阶段。
type memoryStorage struct {
	mu sync.RWMutex

	// 业务 KV 状态机数据
	data   map[string]string
	kvLogs []Command // 业务 KV 的操作日志

	// Raft 相关数据
	raftLogs      []RaftLogEntry // Raft 日志条目
	raftHardState *RaftHardState // 当前 Raft 硬状态（如未设置则为 nil）
}

// Close implements Storage.
func (m *memoryStorage) Close() error {
	// 内存存储无需特殊清理
	return nil
}

// 返回全局的最底层存储实例
func NewStorage(cfg configs.StorageConfig) (Storage, error) {
	return &memoryStorage{
		data:          make(map[string]string),
		kvLogs:        make([]Command, 0),
		raftLogs:      make([]RaftLogEntry, 0),
		raftHardState: nil,
	}, nil
}

func (m *memoryStorage) AppendLog(ctx context.Context, cmd Command) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-ctx.Done():
		return m.LastIndex(), ctx.Err()
	default:
	}

	m.kvLogs = append(m.kvLogs, cmd)
	return uint64(len(m.kvLogs)), nil
}

func (m *memoryStorage) ApplyLog(ctx context.Context, index uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index == 0 || index > uint64(len(m.kvLogs)) {
		return nil
	}
	cmd := m.kvLogs[index-1]

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	switch cmd.Op {
	case "set":
		m.data[cmd.Key] = cmd.Value
	case "delete":
		delete(m.data, cmd.Key)
	}
	return nil
}

func (m *memoryStorage) Get(ctx context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	val, ok := m.data[key]
	if !ok {
		return "", nil
	}
	return val, nil
}

func (m *memoryStorage) LastIndex() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return uint64(len(m.kvLogs))
}

// ===== Raft 日志相关实现（示例内存版本） =====

// AppendRaftLog 追加一批 Raft 日志 entries。
// 要求调用方保证 entries 中的 Index 单调递增且与现有日志连续。
func (m *memoryStorage) AppendRaftLog(ctx context.Context, entries []RaftLogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if len(entries) == 0 {
		return nil
	}

	// 简化：假设 Index 从 1 开始，且不会出现“插入中间”情况。
	m.raftLogs = append(m.raftLogs, entries...)
	return nil
}

// RaftLogEntries 返回 [from, to) 区间内的 Raft 日志；若越界则自动截断。
func (m *memoryStorage) RaftLogEntries(ctx context.Context, from, to uint64) ([]RaftLogEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if from == 0 {
		from = 1
	}
	if from > to || from > uint64(len(m.raftLogs)) {
		return []RaftLogEntry{}, nil
	}

	start := int(from - 1)
	end := int(to - 1)
	if end > len(m.raftLogs) {
		end = len(m.raftLogs)
	}
	if start < 0 {
		start = 0
	}
	if start > end {
		return []RaftLogEntry{}, nil
	}

	res := make([]RaftLogEntry, end-start)
	copy(res, m.raftLogs[start:end])
	return res, nil
}

// RaftLogTerm 返回指定索引的任期；索引不存在时返回 0。
func (m *memoryStorage) RaftLogTerm(ctx context.Context, index uint64) (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	if index == 0 || index > uint64(len(m.raftLogs)) {
		return 0, nil
	}
	return m.raftLogs[index-1].Term, nil
}

// RaftLogLastIndex 返回当前 Raft 日志的最大索引。
func (m *memoryStorage) RaftLogLastIndex(ctx context.Context) (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	return uint64(len(m.raftLogs)), nil
}

// RaftLogTruncateFrom 从 index 起（含）截断 Raft 日志。
func (m *memoryStorage) RaftLogTruncateFrom(ctx context.Context, index uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if index == 0 || index > uint64(len(m.raftLogs)) {
		return nil
	}

	m.raftLogs = m.raftLogs[:index-1]
	return nil
}

// ===== Raft 硬状态相关实现（示例内存版本） =====

func (m *memoryStorage) SaveRaftHardState(ctx context.Context, hs RaftHardState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 覆盖当前内存中的硬状态；真实实现中可以落盘
	m.raftHardState = &hs
	return nil
}

func (m *memoryStorage) LoadRaftHardState(ctx context.Context) (RaftHardState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	select {
	case <-ctx.Done():
		return RaftHardState{}, ctx.Err()
	default:
	}

	if m.raftHardState == nil {
		return RaftHardState{}, nil
	}
	return *m.raftHardState, nil
}
