package chash_grpc

import (
	"context"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/storage"
)

// 实现 CHashServiceServer 接口，处理数据面内部请求。持有的 Storage 应与业务层同源
type ChashGRPCServer struct {
	UnimplementedCHashServiceServer
	st storage.Storage // 底层存储接口
}

// 创建新的 ChashGRPCServer 实例
func NewChashGRPCServer(st storage.Storage) *ChashGRPCServer {
	return &ChashGRPCServer{st: st}
}

// 处理 PullRange RPC 调用
func (s *ChashGRPCServer) PullRange(ctx context.Context, req *PullRangeRequest) (*PullRangeResponse, error) {
	if s.st == nil {
		return &PullRangeResponse{}, errors.ErrResourceNotInit
	}
	kvs, err := s.st.GetHashRange(ctx, req.StartHash, req.EndHash)
	if err != nil {
		return &PullRangeResponse{}, err
	}
	respKVs := make([]*KVPair, 0, len(*kvs))
	for _, kv := range *kvs {
		respKVs = append(respKVs, &KVPair{
			Key:   kv.Key,
			Value: kv.Value,
		})
	}
	return &PullRangeResponse{Kvs: respKVs}, nil
}

// 处理 PushBatch RPC 调用
func (s *ChashGRPCServer) PushBatch(ctx context.Context, req *PushBatchRequest) (*PushBatchResponse, error) {
	if s.st == nil {
		return &PushBatchResponse{Ok: false}, errors.ErrResourceNotInit
	}
	kvs := make([]common.KVPair, 0, len(req.Kvs))
	for _, pbKV := range req.Kvs {
		kvs = append(kvs, common.KVPair{
			Key:   pbKV.Key,
			Value: pbKV.Value,
		})
	}
	// 批量写入底层存储并添加日志
	_, err := s.st.AppendBatchKV(ctx, kvs)
	if err != nil {
		return &PushBatchResponse{Ok: false}, err
	}
	return &PushBatchResponse{Ok: true}, nil
}

// 处理 Replicate RPC 调用
func (s *ChashGRPCServer) Replicate(ctx context.Context, req *ReplicateRequest) (*ReplicateResponse, error) {
	if s.st == nil {
		return &ReplicateResponse{Ok: false}, errors.ErrResourceNotInit
	}
	cmds := make([]common.Command, 0, len(req.Cmds))
	for _, pbCmd := range req.Cmds {
		var op common.CommandOperation
		switch pbCmd.Op {
		case CommandOperation_OP_PUT:
			op = common.OpPut
		case CommandOperation_OP_DELETE:
			op = common.OpDelete
		default:
			return &ReplicateResponse{Ok: false}, errors.ErrInvalidCommandOp
		}
		cmds = append(cmds, common.Command{
			Op:    op,
			Key:   pbCmd.Key,
			Value: pbCmd.Value,
		})
	}
	// 批量应用到底层状态机
	err := s.st.BatchApply(ctx, cmds)
	if err != nil {
		return &ReplicateResponse{Ok: false}, err
	}
	return &ReplicateResponse{Ok: true}, nil
}
