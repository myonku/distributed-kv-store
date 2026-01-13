package services

import (
	"context"
	"distributed-kv-store/internal/bridge"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/storage"
)

// 基于Gossip + 一致性哈希模式的 KVService 实现
type CHashKVService struct {
	memberBridge *bridge.MemberBridge // 持有 gossip 节点和一致性哈希环实例
	st           storage.Storage      // 本地存储
}

func NewCHashKVService(memberBridge *bridge.MemberBridge, st storage.Storage) KVService {
	return &CHashKVService{memberBridge: memberBridge, st: st}
}

func (s *CHashKVService) Put(ctx context.Context, key, value string) error {
	nodes, err := s.memberBridge.OwnerNodeIDs(key)
	if err != nil {
		return err
	}
	// 将请求发送到所有负责该 key 的节点
	for _, nodeID := range nodes {
		if nodeID == s.memberBridge.SelfID() {
			cmd := common.Command{
				Op:    common.OpPut,
				Key:   key,
				Value: value,
			}
			index, err := s.st.AppendLog(ctx, cmd)
			if err != nil {
				return err
			}
			if err := s.st.ApplyLog(ctx, index); err != nil {
				return err
			}
		} else {
			if err := s.memberBridge.ForwardPut(ctx, nodeID, key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *CHashKVService) Get(ctx context.Context, key string) (string, error) {
	nodes, err := s.memberBridge.OwnerNodeIDs(key)
	if err != nil {
		return "", err
	}
	// 尝试从负责该 key 的节点中读取，读取到任意一个节点的数据即可返回
	for _, nodeID := range nodes {
		if nodeID == s.memberBridge.SelfID() {
			if value, err := s.st.Get(ctx, key); err == nil {
				return value, nil
			}
		} else {
			value, err := s.memberBridge.ForwardGet(ctx, nodeID, key)
			if err == nil {
				return value, nil
			}
		}
	}
	return "", errors.Error{Type: errors.ObjectNotFound, Info: "key not found"}
}

func (s *CHashKVService) Delete(ctx context.Context, key string) error {
	nodes, err := s.memberBridge.OwnerNodeIDs(key)
	// 将请求发送到所有负责该 key 的节点
	if err != nil {
		return err
	}
	for _, nodeID := range nodes {
		if nodeID == s.memberBridge.SelfID() {
			cmd := common.Command{
				Op:  common.OpDelete,
				Key: key,
			}
			index, err := s.st.AppendLog(ctx, cmd)
			if err != nil {
				return err
			}
			if err := s.st.ApplyLog(ctx, index); err != nil {
				return err
			}
		} else {
			if err := s.memberBridge.ForwardDelete(ctx, nodeID, key); err != nil {
				return err
			}
		}
	}
	return nil
}

// 支持在外部启动 MemberBridge
func (s *CHashKVService) RunService() {
	if s.memberBridge == nil || s.memberBridge.IsRunning() {
		return
	}
	s.memberBridge.Start()
}

// 释放资源，停止 MemberBridge
func (s *CHashKVService) Dispose() {
	if s.memberBridge == nil || !s.memberBridge.IsRunning() {
		return
	}
	s.memberBridge.Stop()
}
