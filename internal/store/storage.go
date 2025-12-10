package store

import (
	"context"
	"sync"

	"distributed-kv-store/configs"
)

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
