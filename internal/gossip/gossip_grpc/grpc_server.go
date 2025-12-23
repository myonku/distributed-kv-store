package gossip_grpc

import (
	context "context"
	"distributed-kv-store/internal/gossip"

	"google.golang.org/grpc"
)

// GossipServiceServer 的实现，内部持有 *gossip.Node。成员合并/冲突解决策略放在 gossip 包内实现
type GossipGRPCServer struct {
	UnimplementedGossipServiceServer
	node *gossip.Node
}

func NewGossipGRPCServer(node *gossip.Node) *GossipGRPCServer {
	return &GossipGRPCServer{node: node}
}

// 生成一个标准 grpc.Server，供外部统一创建
func NewGRPCServerWrapper() *grpc.Server {
	return grpc.NewServer()
}

// 处理 Ping RPC 调用
func (s *GossipGRPCServer) Ping(ctx context.Context, req *PingRequest) (*PingResponse, error) {
	internalReq := &gossip.PingRequest{
		FromID:          req.FromId,
		FromIncarnation: req.FromIncarnation,
	}

	resp, err := s.node.HandlePing(ctx, internalReq)
	if err != nil {
		return nil, err
	}
	return &PingResponse{Ok: resp.OK}, nil
}

// 处理 PushPull RPC 调用
func (s *GossipGRPCServer) PushPull(ctx context.Context, req *PushPullRequest) (*PushPullResponse, error) {
	internalReq := &gossip.PushPullRequest{FromID: req.FromId}

	// Digests
	if len(req.Digests) > 0 {
		internalReq.Digests = make([]gossip.Digest, 0, len(req.Digests))
		for _, d := range req.Digests {
			internalReq.Digests = append(internalReq.Digests, gossip.Digest{
				ID:          d.Id,
				Incarnation: d.Incarnation,
				State:       fromPBState(d.State),
			})
		}
	}

	// FullMembers
	if len(req.FullMembers) > 0 {
		internalReq.FullMembers = make([]gossip.Member, 0, len(req.FullMembers))
		for _, m := range req.FullMembers {
			internalReq.FullMembers = append(internalReq.FullMembers, gossip.Member{
				ID:              m.Id,
				InternalAddress: m.InternalAddress,
				ClientAddress:   m.ClientAddress,
				Weight:          int(m.Weight),
				State:           fromPBState(m.State),
				Incarnation:     m.Incarnation,
				StateUpdated:    m.StateUpdated,
			})
		}
	}

	resp, err := s.node.HandlePushPull(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	pbResp := &PushPullResponse{}
	if len(resp.Delta) > 0 {
		pbResp.Delta = make([]*Member, 0, len(resp.Delta))
		for _, m := range resp.Delta {
			pbResp.Delta = append(pbResp.Delta, &Member{
				Id:              m.ID,
				InternalAddress: m.InternalAddress,
				ClientAddress:   m.ClientAddress,
				Weight:          int32(m.Weight),
				State:           toPBState(m.State),
				Incarnation:     m.Incarnation,
				StateUpdated:    m.StateUpdated,
			})
		}
	}
	return pbResp, nil
}

// 状态转换辅助函数
func toPBState(s gossip.NodeState) NodeState {
	switch s {
	case gossip.StateSuspect:
		return NodeState_NODE_STATE_SUSPECT
	case gossip.StateDead:
		return NodeState_NODE_STATE_DEAD
	case gossip.StateAlive:
		fallthrough
	default:
		return NodeState_NODE_STATE_ALIVE
	}
}

// 状态转换辅助函数
func fromPBState(s NodeState) gossip.NodeState {
	switch s {
	case NodeState_NODE_STATE_SUSPECT:
		return gossip.StateSuspect
	case NodeState_NODE_STATE_DEAD:
		return gossip.StateDead
	case NodeState_NODE_STATE_ALIVE:
		fallthrough
	default:
		return gossip.StateAlive
	}
}
