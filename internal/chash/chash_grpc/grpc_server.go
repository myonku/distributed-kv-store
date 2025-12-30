package chash_grpc

import (
	"context"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/storage"
)

// ChashGRPCServer 实现了 CHashServiceServer 接口，处理数据面内部请求。
// 内部持有的 storage.Storage 应与业务层同源
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
	cmds, err := s.st.GetBatch(ctx, req.StartIndex, req.EndIndex)
	if err != nil {
		return &PullRangeResponse{}, err
	}
	respCmds := make([]*Command, 0, len(*cmds))
	for _, cmd := range *cmds {
		var pbOp CommandOperation
		switch cmd.Op {
		case storage.OpPut:
			pbOp = CommandOperation_OP_PUT
		case storage.OpDelete:
			pbOp = CommandOperation_OP_DELETE
		default:
			return &PullRangeResponse{}, errors.ErrInvalidCommandOp
		}
		respCmds = append(respCmds, &Command{
			Op:    pbOp,
			Key:   cmd.Key,
			Value: cmd.Value,
		})
	}
	return &PullRangeResponse{Cmds: respCmds}, nil
}

// 处理 PushBatch RPC 调用
func (s *ChashGRPCServer) PushBatch(ctx context.Context, req *PushBatchRequest) (*PushBatchResponse, error) {
	if s.st == nil {
		return &PushBatchResponse{Ok: false}, errors.ErrResourceNotInit
	}
	cmds := make([]storage.Command, 0, len(req.Cmds))
	for _, pbCmd := range req.Cmds {
		var op storage.CommandOperation
		switch pbCmd.Op {
		case CommandOperation_OP_PUT:
			op = storage.OpPut
		case CommandOperation_OP_DELETE:
			op = storage.OpDelete
		default:
			return &PushBatchResponse{Ok: false}, errors.ErrInvalidCommandOp
		}
		cmds = append(cmds, storage.Command{
			Op:    op,
			Key:   pbCmd.Key,
			Value: pbCmd.Value,
		})
	}
	// 批量应用命令
	err := s.st.BatchApply(ctx, cmds)
	if err != nil {
		return &PushBatchResponse{Ok: false}, err
	}
	return &PushBatchResponse{Ok: true}, nil
}

// 处理 Replicate RPC 调用
func (s *ChashGRPCServer) Replicate(ctx context.Context, req *ReplicateRequest) (*ReplicateResponse, error) {
	// 内部实现暂时与 PushBatch 相同
	if s.st == nil {
		return &ReplicateResponse{Ok: false}, errors.ErrResourceNotInit
	}
	cmds := make([]storage.Command, 0, len(req.Cmds))
	for _, pbCmd := range req.Cmds {
		var op storage.CommandOperation
		switch pbCmd.Op {
		case CommandOperation_OP_PUT:
			op = storage.OpPut
		case CommandOperation_OP_DELETE:
			op = storage.OpDelete
		default:
			return &ReplicateResponse{Ok: false}, errors.ErrInvalidCommandOp
		}
		cmds = append(cmds, storage.Command{
			Op:    op,
			Key:   pbCmd.Key,
			Value: pbCmd.Value,
		})
	}
	err := s.st.BatchApply(ctx, cmds)
	if err != nil {
		return &ReplicateResponse{Ok: false}, err
	}
	return &ReplicateResponse{Ok: true}, nil
}
