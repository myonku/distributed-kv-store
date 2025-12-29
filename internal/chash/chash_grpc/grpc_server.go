package chash_grpc

import (
	"context"
	"distributed-kv-store/internal/storage"
)

// ChashGRPCServer 实现了 CHashServiceServer 接口，处理数据面内部请求。
// 内部持有的 storage.Storage 不能独立于业务层使用
type ChashGRPCServer struct {
	UnimplementedCHashServiceServer
	st storage.Storage // 底层存储接口
}

// 创建新的 ChashGRPCServer 实例
func NewChashGRPCServer(st storage.Storage) *ChashGRPCServer {
	return &ChashGRPCServer{st: st}
}

// 处理 Replicate RPC 调用
func (s *ChashGRPCServer) PullRange(context.Context, *PullRangeRequest) (*PullRangeResponse, error) {
	return &PullRangeResponse{}, nil
}

// 处理 PushBatch RPC 调用
func (s *ChashGRPCServer) PushBatch(context.Context, *PushBatchRequest) (*PushBatchResponse, error) {
	return &PushBatchResponse{}, nil
}

// 处理 Replicate RPC 调用
func (s *ChashGRPCServer) Replicate(context.Context, *ReplicateRequest) (*ReplicateResponse, error) {
	return &ReplicateResponse{}, nil
}
