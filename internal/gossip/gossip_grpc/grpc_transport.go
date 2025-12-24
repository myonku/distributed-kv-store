package gossip_grpc

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/gossip"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 最小实现用于节点间 Ping/PushPull。
type GRPCTransport struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn    // peerID -> conn
	cli   map[string]GossipServiceClient // peerID -> client
}

// 返回新的 Gossip GRPCTransport 实例
func NewGRPCTransport(peers []configs.ClusterNode) (*GRPCTransport, error) {
	t := &GRPCTransport{
		conns: make(map[string]*grpc.ClientConn),
		cli:   make(map[string]GossipServiceClient),
	}

	for _, p := range peers {
		options := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		conn, err := grpc.NewClient(p.GossipGRPCAddress, options...)
		if err != nil {
			return nil, fmt.Errorf("dial %s: %w", p.GossipGRPCAddress, err)
		}
		t.conns[p.ID] = conn
		t.cli[p.ID] = NewGossipServiceClient(conn)
	}

	return t, nil
}

// 发送 Ping 消息
func (t *GRPCTransport) Ping(ctx context.Context, to string, req *gossip.PingRequest) (*gossip.PingResponse, error) {
	t.mu.RLock()
	client, ok := t.cli[to]
	t.mu.RUnlock()
	if !ok {
		return nil, errors.ErrClientNotExist
	}

	pbReq := &PingRequest{
		FromId:          req.FromID,
		FromIncarnation: req.FromIncarnation,
	}

	pbResp, err := client.Ping(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return &gossip.PingResponse{OK: pbResp.Ok}, nil
}

// 发送 PushPull 消息
func (t *GRPCTransport) PushPull(
	ctx context.Context,
	to string,
	req *gossip.PushPullRequest) (*gossip.PushPullResponse, error) {

	t.mu.RLock()
	client, ok := t.cli[to]
	t.mu.RUnlock()
	if !ok {
		return nil, errors.ErrClientNotExist
	}

	pbReq := &PushPullRequest{FromId: req.FromID}

	if len(req.Digests) > 0 {
		pbReq.Digests = make([]*Digest, 0, len(req.Digests))
		for _, d := range req.Digests {
			pbReq.Digests = append(pbReq.Digests, &Digest{
				Id:          d.ID,
				Incarnation: d.Incarnation,
				State:       toPBState(d.State),
			})
		}
	}

	if len(req.FullMembers) > 0 {
		pbReq.FullMembers = make([]*Member, 0, len(req.FullMembers))
		for _, m := range req.FullMembers {
			pbReq.FullMembers = append(pbReq.FullMembers, &Member{
				Id:                m.ID,
				GossipGrpcAddress: m.GossipGRPCAddress,
				ClientAddress:     m.ClientAddress,
				Weight:            int32(m.Weight),
				State:             toPBState(m.State),
				Incarnation:       m.Incarnation,
				StateUpdated:      m.StateUpdated,
			})
		}
	}

	pbResp, err := client.PushPull(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	resp := &gossip.PushPullResponse{}
	if len(pbResp.Delta) > 0 {
		resp.Delta = make([]gossip.Member, 0, len(pbResp.Delta))
		for _, m := range pbResp.Delta {
			resp.Delta = append(resp.Delta, gossip.Member{
				ID:                m.Id,
				GossipGRPCAddress: m.GossipGrpcAddress,
				ClientAddress:     m.ClientAddress,
				Weight:            int(m.Weight),
				State:             fromPBState(m.State),
				Incarnation:       m.Incarnation,
				StateUpdated:      m.StateUpdated,
			})
		}
	}

	return resp, nil
}

// 添加新的 Peer 连接
func (t *GRPCTransport) AddPeer(peer configs.ClusterNode) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.cli[peer.ID]; exists {
		_ = t.conns[peer.ID].Close()
		delete(t.conns, peer.ID)
		delete(t.cli, peer.ID)
	}

	options := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	conn, err := grpc.NewClient(peer.GossipGRPCAddress, options...)
	if err != nil {
		return fmt.Errorf("dial %s: %w", peer.GossipGRPCAddress, err)
	}

	t.conns[peer.ID] = conn
	t.cli[peer.ID] = NewGossipServiceClient(conn)
	return nil
}

// 移除 Peer 连接
func (t *GRPCTransport) RemovePeer(peerID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	conn, exists := t.conns[peerID]
	if !exists {
		return nil
	}

	if err := conn.Close(); err != nil {
		return err
	}

	delete(t.conns, peerID)
	delete(t.cli, peerID)
	return nil
}

func (t *GRPCTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, c := range t.conns {
		_ = c.Close()
	}
	return nil
}
