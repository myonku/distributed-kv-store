package chash_grpc

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/storage"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// 最小实现用于节点间 PushBatch/PullRange/Replicate
type GRPCTransport struct {
	mu    sync.RWMutex
	conns map[string]*grpc.ClientConn   // peerID -> conn
	cli   map[string]CHashServiceClient // peerID -> client
}

// 返回新的 CHash GRPCTransport 实例
func NewGRPCTransport() chash.Transport {
	return &GRPCTransport{
		conns: make(map[string]*grpc.ClientConn),
		cli:   make(map[string]CHashServiceClient),
	}
}

// PushBatch 实现
func (t *GRPCTransport) PushBatch(ctx context.Context, to string, cmds *[]storage.Command) error {
	t.mu.RLock()
	client, ok := t.cli[to]
	t.mu.RUnlock()
	if !ok {
		return errors.ErrClientNotExist
	}

	pbReq := &PushBatchRequest{
		Cmds: make([]*Command, 0, len(*cmds)),
	}
	for _, cmd := range *cmds {
		var pbOp CommandOperation
		switch cmd.Op {
		case storage.OpPut:
			pbOp = CommandOperation_OP_PUT
		case storage.OpDelete:
			pbOp = CommandOperation_OP_DELETE
		default:
			return errors.ErrInvalidCommandOp
		}
		pbReq.Cmds = append(pbReq.Cmds, &Command{
			Op:    pbOp,
			Key:   cmd.Key,
			Value: cmd.Value,
		})
	}
	resp, err := client.PushBatch(ctx, pbReq)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.ErrPushBatchFailed
	}
	return nil
}

// PullRange 实现
func (t *GRPCTransport) PullRange(ctx context.Context, to string, startIndex, endIndex uint64) (*[]storage.Command, error) {
	t.mu.RLock()
	client, ok := t.cli[to]
	t.mu.RUnlock()
	if !ok {
		return nil, errors.ErrClientNotExist
	}
	pbReq := &PullRangeRequest{
		StartIndex: startIndex,
		EndIndex:   endIndex,
	}
	resp, err := client.PullRange(ctx, pbReq)
	if err != nil {
		return nil, err
	}
	cmds := make([]storage.Command, 0, len(resp.Cmds))
	for _, pbCmd := range resp.Cmds {
		var op storage.CommandOperation
		switch pbCmd.Op {
		case CommandOperation_OP_PUT:
			op = storage.OpPut
		case CommandOperation_OP_DELETE:
			op = storage.OpDelete
		default:
			return nil, errors.ErrInvalidCommandOp
		}
		cmds = append(cmds, storage.Command{
			Op:    op,
			Key:   pbCmd.Key,
			Value: pbCmd.Value,
		})
	}

	return &cmds, nil
}

// Replicate 实现
func (t *GRPCTransport) Replicate(ctx context.Context, to string, cmds *[]storage.Command) error {
	t.mu.RLock()
	client, ok := t.cli[to]
	t.mu.RUnlock()
	if !ok {
		return errors.ErrClientNotExist
	}
	pbReq := &ReplicateRequest{
		Cmds: make([]*Command, 0, len(*cmds)),
	}
	for _, cmd := range *cmds {
		var pbOp CommandOperation
		switch cmd.Op {
		case storage.OpPut:
			pbOp = CommandOperation_OP_PUT
		case storage.OpDelete:
			pbOp = CommandOperation_OP_DELETE
		default:
			return errors.ErrInvalidCommandOp
		}
		pbReq.Cmds = append(pbReq.Cmds, &Command{
			Op:    pbOp,
			Key:   cmd.Key,
			Value: cmd.Value,
		})
	}
	resp, err := client.Replicate(ctx, pbReq)
	if err != nil {
		return err
	}
	if !resp.Ok {
		return errors.ErrReplicateFailed
	}
	return nil
}

// 新增连接
func (t *GRPCTransport) AddConnection(peer configs.ClusterNode, options ...grpc.DialOption) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, exists := t.cli[peer.ID]; exists {
		// 已存在，覆盖连接
		_ = t.conns[peer.ID].Close()
		delete(t.conns, peer.ID)
		delete(t.cli, peer.ID)
	}
	if options == nil {
		options = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}
	conn, err := grpc.NewClient(peer.CHashGRPCAddress, options...)
	if err != nil {
		return err
	}
	t.conns[peer.ID] = conn
	t.cli[peer.ID] = NewCHashServiceClient(conn)
	return nil
}

// 移除连接
func (t *GRPCTransport) RemoveConnection(peerID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if conn, ok := t.conns[peerID]; ok {
		conn.Close()
		delete(t.conns, peerID)
		delete(t.cli, peerID)
	}
	return nil
}

// 关闭所有连接
func (t *GRPCTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, conn := range t.conns {
		conn.Close()
	}
	return nil
}
