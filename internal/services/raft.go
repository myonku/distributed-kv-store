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
			return errors.Error{Type: errors.ObjectNotFound, Info: "leader does not exist"}
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
			return errors.Error{Type: errors.ObjectNotFound, Info: "leader does not exist"}
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
			return "", errors.Error{Type: errors.ObjectNotFound, Info: "leader does not exist"}
		}
	}
	// 确保线性一致性读
	if err := s.node.LinearizableRead(ctx); err != nil {
		return "", err
	}
	if value, ok, err := s.st.Get(ctx, key); err != nil {
		return "", err
	} else if !ok {
		return "", errors.Error{Type: errors.ObjectNotFound, Info: "key not found"}
	} else {
		return value, nil
	}
}

// 支持在外部启动 Node
func (s *RaftKVService) RunService() {
	if s.node == nil || s.node.IsRunning() {
		return
	}
	s.node.Start()
}

// 释放资源，停止 Raft 节点
func (s *RaftKVService) Dispose() {
	if s.node == nil || !s.node.IsRunning() {
		return
	}
	s.node.Stop()
}
