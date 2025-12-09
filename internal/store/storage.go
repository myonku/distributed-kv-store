package store

import (
	"context"
	"sync"

	"distributed-kv-store/configs"
)

// 表示对底层状态机的一个逻辑操作。
// 无论是 Raft 日志 entry 还是一致性哈希节点上的本地写入，都可以统一抽象为 Command。
type Command struct {
	Op    string
	Key   string
	Value string
}

// 对底层 KV 状态机 + 持久化的抽象。
type Storage interface {
	AppendLog(ctx context.Context, cmd Command) (index uint64, err error)
	ApplyLog(ctx context.Context, index uint64) error
	Get(ctx context.Context, key string) (string, error)
	LastIndex() uint64
}

// 供外部 Service 层使用的封装
type KVStore interface {
	Put(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
}

// 内存实现 + 简单的 index 递增，用于单机/早期开发阶段。
type memoryStorage struct {
	mu   sync.RWMutex
	data map[string]string
	logs []Command
}

func NewStorage(cfg configs.StorageConfig) (Storage, error) {
	return &memoryStorage{
		data: make(map[string]string),
		logs: make([]Command, 0),
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

	m.logs = append(m.logs, cmd)
	return uint64(len(m.logs)), nil
}

func (m *memoryStorage) ApplyLog(ctx context.Context, index uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if index == 0 || index > uint64(len(m.logs)) {
		return nil
	}
	cmd := m.logs[index-1]

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
	return uint64(len(m.logs))
}
