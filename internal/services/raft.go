package services

import (
	"context"

	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/storage"
)

// 基于 Raft 的分布式 KVService 实现
type RaftKVService struct {
	st           storage.Storage
	node         *raft.Node
	remoteClient common.RemoteClient
}

func NewRaftKVService(st storage.Storage, node *raft.Node, remoteClient common.RemoteClient) KVService {
	return &RaftKVService{st: st, node: node, remoteClient: remoteClient}
}

func (s *RaftKVService) Put(ctx context.Context, key, value string) error {
	if !s.node.IsLeader() {
		currentLeader := s.node.LeaderInfo()
		if currentLeader.ClientAddress != "" {
			// 转发请求到 Leader 节点
			return s.remoteClient.Put(ctx, currentLeader.ClientAddress, key, value)
		} else {
			return errors.ErrLeaderDoesNotExist
		}
	}

	cmd := common.Command{
		Op:    common.OpPut,
		Key:   key,
		Value: value,
	}
	_, err := s.node.Propose(ctx, cmd)
	return err
}

func (s *RaftKVService) Delete(ctx context.Context, key string) error {
	if !s.node.IsLeader() {
		currentLeader := s.node.LeaderInfo()
		if currentLeader.ClientAddress != "" {
			// 转发请求到 Leader 节点
			return s.remoteClient.Delete(ctx, currentLeader.ClientAddress, key)
		} else {
			return errors.ErrLeaderDoesNotExist
		}
	}

	cmd := common.Command{
		Op:  common.OpDelete,
		Key: key,
	}
	_, err := s.node.Propose(ctx, cmd)
	return err
}

func (s *RaftKVService) Get(ctx context.Context, key string) (string, error) {
	if !s.node.IsLeader() {
		currentLeader := s.node.LeaderInfo()
		if currentLeader.ClientAddress != "" {
			// 转发请求到 Leader 节点
			return s.remoteClient.Get(ctx, currentLeader.ClientAddress, key)
		} else {
			return "", errors.ErrLeaderDoesNotExist
		}
	}
	// 确保线性一致性读
	if err := s.node.LinearizableRead(ctx); err != nil {
		return "", err
	}
	return s.st.Get(ctx, key)
}
