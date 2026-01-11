package services

import (
	"context"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/storage"
)

// 单机模式下的 KVService 实现。
type StandaloneKVService struct {
	st storage.Storage
}

func NewStandaloneKVService(st storage.Storage) KVService {
	return &StandaloneKVService{st: st}
}

func (s *StandaloneKVService) RunService() {
	// 单机模式下无需特殊处理
}

func (s *StandaloneKVService) Dispose() {
	// 单机模式下无需特殊处理
}

func (s *StandaloneKVService) Put(ctx context.Context, key, value string) error {
	cmd := common.Command{
		Op:    common.OpPut,
		Key:   key,
		Value: value,
	}
	idx, err := s.st.AppendLog(ctx, cmd)
	if err != nil {
		return err
	}
	err = s.st.ApplyLog(ctx, idx)
	return err
}

func (s *StandaloneKVService) Get(ctx context.Context, key string) (string, error) {
	return s.st.Get(ctx, key)
}

func (s *StandaloneKVService) Delete(ctx context.Context, key string) error {
	cmd := common.Command{
		Op:  common.OpDelete,
		Key: key,
	}
	idx, err := s.st.AppendLog(ctx, cmd)
	if err != nil {
		return err
	}
	err = s.st.ApplyLog(ctx, idx)
	return err
}
