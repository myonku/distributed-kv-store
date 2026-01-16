package chash_grpc

import (
	"context"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/storage"
)

// 实现 CHashServiceServer 接口，处理数据面内部请求。持有的 Storage 应与业务层同源
type ChashGRPCServer struct {
	UnimplementedCHashServiceServer
	st   storage.Storage // 底层存储接口
	ring chash.Ring      // 所属一致性哈希环实例
}

// 创建新的 ChashGRPCServer 实例
func NewChashGRPCServer(st storage.Storage, ring chash.Ring) *ChashGRPCServer {
	return &ChashGRPCServer{st: st, ring: ring}
}

// 处理 PullRange RPC 调用
func (s *ChashGRPCServer) PullRange(ctx context.Context, req *PullRangeRequest) (*PullRangeResponse, error) {
	if s.st == nil {
		return &PullRangeResponse{}, errors.Error{Type: errors.ImportError, Info: "resource not initialized"}
	}
	// PullRange 是“源节点读数据”的操作：为了支持失败重试/重复拉取，这里不做 moveID 去重
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
		return &PushBatchResponse{Ok: false}, errors.Error{Type: errors.ImportError, Info: "resource not initialized"}
	}
	// moveID 已存在则表示该迁移已完成，直接返回 OK（幂等）
	if exists, err := s.st.GetMoveRangeRecord(ctx, req.MoveId); err != nil {
		return &PushBatchResponse{Ok: false}, err
	} else if exists {
		return &PushBatchResponse{Ok: true}, nil
	}

	kvs := make([]common.KVPair, 0, len(req.Kvs))
	for _, pbKV := range req.Kvs {
		kvs = append(kvs, common.KVPair{
			Key:   pbKV.Key,
			Value: pbKV.Value,
		})
	}
	// 批量写入底层存储并添加日志
	_, err := s.st.AppendBatchKV(ctx, &kvs)
	if err != nil {
		return &PushBatchResponse{Ok: false}, err
	}
	// 仅在写入成功后记录 moveID，避免失败后误判“已完成”导致无法重试
	if err := s.st.SaveMoveRangeRecord(ctx, req.MoveId); err != nil {
		return &PushBatchResponse{Ok: false}, err
	}
	return &PushBatchResponse{Ok: true}, nil
}

// 处理 Replicate RPC 调用
func (s *ChashGRPCServer) Replicate(ctx context.Context, req *ReplicateRequest) (*ReplicateResponse, error) {
	if s.st == nil {
		return &ReplicateResponse{Ok: false}, errors.Error{Type: errors.ImportError, Info: "resource not initialized"}
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
			return &ReplicateResponse{Ok: false}, errors.Error{Type: errors.InvalidArgument, Info: "invalid command operation"}
		}
		cmds = append(cmds, common.Command{
			Op:    op,
			Key:   pbCmd.Key,
			Value: pbCmd.Value,
		})
	}
	// 批量应用到底层状态机
	err := s.st.BatchApply(ctx, &cmds)
	if err != nil {
		return &ReplicateResponse{Ok: false}, err
	}
	return &ReplicateResponse{Ok: true}, nil
}

// 处理 AnnouncePlan RPC 调用
func (s *ChashGRPCServer) AnnouncePlan(ctx context.Context, req *AnnouncePlanRequest) (*AckPlan, error) {
	if s.ring == nil {
		return &AckPlan{Ok: false}, errors.Error{Type: errors.ImportError, Info: "ring not initialized"}
	}
	plans := make([]chash.MovePlanHint, 0, len(req.Plans))
	for _, pbPlan := range req.Plans {
		plans = append(plans, chash.MovePlanHint{
			Epoch:     pbPlan.Epoch,
			StartHash: pbPlan.StartHash,
			EndHash:   pbPlan.EndHash,
			OldOwners: pbPlan.OldOwners,
			NewOwners: pbPlan.NewOwners,
			Status:    chash.MigrationStatus(pbPlan.Status),
		})
	}
	s.ring.RecordPlanHints(&plans)
	return &AckPlan{Ok: true}, nil
}

// 处理 PullPlanSince RPC 调用
func (s *ChashGRPCServer) PullPlanSince(ctx context.Context, req *PullPlanSinceRequest) (*PullPlanSinceResponse, error) {
	if s.ring == nil {
		return &PullPlanSinceResponse{Plans: []*MovePlanHint{}}, errors.Error{Type: errors.ImportError, Info: "ring not initialized"}
	}
	plans := s.ring.PlanHintsSince(req.SinceEpoch)
	pbPlans := make([]*MovePlanHint, 0, len(*plans))
	for _, plan := range *plans {
		pbPlans = append(pbPlans, &MovePlanHint{
			Epoch:     plan.Epoch,
			StartHash: plan.StartHash,
			EndHash:   plan.EndHash,
			OldOwners: plan.OldOwners,
			NewOwners: plan.NewOwners,
			Status:    MigrationStatus(plan.Status),
		})
	}
	return &PullPlanSinceResponse{Plans: pbPlans}, nil
}
