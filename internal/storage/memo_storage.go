package storage

import (
	"context"
	"sync"
	"time"

	"distributed-kv-store/configs"
	"distributed-kv-store/internal/common"
)

// 内存实现 + 简单的 index 递增
type memoryStorage struct {
	mu sync.RWMutex

	// 业务 KV 状态机数据

	data      map[string]string
	kvLogs    []common.Command  // 业务 KV 的操作日志
	hashIndex map[string]uint32 // 记录每个 key 对应的哈希值，方便按哈希范围查询

	// moveRangeRecords 用于迁移去重记录，避免与业务 KV 命名空间混用。
	moveRangeRecords map[uint32]string

	// Raft 相关数据

	raftLogs      []common.RaftLogEntry // Raft 日志条目
	raftHardState *common.RaftHardState // 当前 Raft 硬状态（如未设置则为 nil）
}

func NewStorage(cfg configs.StorageConfig) (Storage, error) {
	return &memoryStorage{
		data:             make(map[string]string),
		kvLogs:           make([]common.Command, 0),
		hashIndex:        make(map[string]uint32),
		moveRangeRecords: make(map[uint32]string),
		raftLogs:         make([]common.RaftLogEntry, 0),
		raftHardState:    nil,
	}, nil
}

func (m *memoryStorage) AppendBatchKV(ctx context.Context, kvs *[]common.KVPair) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-ctx.Done():
		return m.LastIndex(), ctx.Err()
	default:
	}
	startIndex := uint64(len(m.kvLogs) + 1)
	for _, kv := range *kvs {
		m.data[kv.Key] = kv.Value
		hashVal := common.HashKey(kv.Key)
		m.hashIndex[kv.Key] = hashVal
		m.kvLogs = append(m.kvLogs, common.Command{
			Op:    common.OpPut,
			Key:   kv.Key,
			Value: kv.Value,
		})
	}
	return startIndex, nil
}

func (m *memoryStorage) GetHashRange(ctx context.Context, startHash, endHash uint32) (*[]common.KVPair, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pairs := make([]common.KVPair, 0)
	for key, hasVal := range m.hashIndex {
		inRange := false
		if startHash == endHash {
			inRange = false
		} else if startHash < endHash {
			inRange = hasVal >= startHash && hasVal < endHash
		} else {
			// wrap-around: [startHash, 2^32) U [0, endHash)
			inRange = hasVal >= startHash || hasVal < endHash
		}
		if inRange {
			// 包含在范围内
			val := m.data[key]
			pairs = append(pairs, common.KVPair{
				Key:   key,
				Value: val,
			})
		}
	}
	return &pairs, nil
}

func (m *memoryStorage) GetMoveRangeRecord(ctx context.Context, moveID uint32) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	_, exists := m.moveRangeRecords[moveID]
	return exists, nil
}

func (m *memoryStorage) SaveMoveRangeRecord(ctx context.Context, moveID uint32) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	// 使用当前时间戳作为记录内容，方便后续过期清理
	m.moveRangeRecords[moveID] = time.Now().Format(time.RFC3339)
	return nil
}

func (m *memoryStorage) Close() error {
	return nil
}

func (m *memoryStorage) AppendLog(ctx context.Context, cmd common.Command) (uint64, error) {
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
	case common.OpPut:
		m.data[cmd.Key] = cmd.Value
		hashVal := common.HashKey(cmd.Key)
		m.hashIndex[cmd.Key] = hashVal
	case common.OpDelete:
		delete(m.data, cmd.Key)
		delete(m.hashIndex, cmd.Key)
	case common.OpNoop:
	}
	return nil
}

func (m *memoryStorage) BatchApply(ctx context.Context, cmds *[]common.Command) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	for _, cmd := range *cmds {
		switch cmd.Op {
		case common.OpPut:
			m.data[cmd.Key] = cmd.Value
			hashVal := common.HashKey(cmd.Key)
			m.hashIndex[cmd.Key] = hashVal
		case common.OpDelete:
			delete(m.data, cmd.Key)
			delete(m.hashIndex, cmd.Key)
		case common.OpNoop:
		}
		m.kvLogs = append(m.kvLogs, cmd)
	}
	return nil
}

func (m *memoryStorage) GetBatch(ctx context.Context, startIndex, endIndex uint64) (*[]common.Command, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if startIndex == 0 {
		startIndex = 1
	}
	if startIndex > endIndex || startIndex > uint64(len(m.kvLogs)) {
		return &[]common.Command{}, nil
	}
	start := int(startIndex - 1)
	end := min(int(endIndex-1), len(m.kvLogs))
	if start < 0 {
		start = 0
	}
	if start > end {
		return &[]common.Command{}, nil
	}
	res := make([]common.Command, end-start)
	copy(res, m.kvLogs[start:end])
	return &res, nil
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

// 追加一批 Raft 日志 entries.要求调用方保证 entries 中的 Index 单调递增且与现有日志连续
func (m *memoryStorage) AppendRaftLog(ctx context.Context, entries []common.RaftLogEntry) error {
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

	// 假设 Index 从 1 开始，且不会出现“插入中间”情况。
	m.raftLogs = append(m.raftLogs, entries...)
	return nil
}

// RaftLogEntries 返回 [from, to) 区间内的 Raft 日志；若越界则自动截断
func (m *memoryStorage) RaftLogEntries(ctx context.Context, from, to uint64) ([]common.RaftLogEntry, error) {
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
		return []common.RaftLogEntry{}, nil
	}

	start := int(from - 1)
	end := min(int(to-1), len(m.raftLogs))
	if start < 0 {
		start = 0
	}
	if start > end {
		return []common.RaftLogEntry{}, nil
	}

	res := make([]common.RaftLogEntry, end-start)
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

// RaftLogLastIndex 返回当前 Raft 日志的最大索引
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

// RaftLogTruncateFrom 从 index 起（含）截断 Raft 日志
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

func (m *memoryStorage) SaveRaftHardState(ctx context.Context, hs common.RaftHardState) error {
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

func (m *memoryStorage) LoadRaftHardState(ctx context.Context) (common.RaftHardState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	select {
	case <-ctx.Done():
		return common.RaftHardState{}, ctx.Err()
	default:
	}

	if m.raftHardState == nil {
		return common.RaftHardState{}, nil
	}
	return *m.raftHardState, nil
}
