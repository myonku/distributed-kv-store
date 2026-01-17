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
	primary, hinted, err := s.memberBridge.ResolveWriteOwners(key)
	if err != nil {
		return err
	}
	// 将请求发送到所有负责该 key 的节点（优先 primary，再补充 hinted）
	seen := make(map[string]struct{})
	// 优先写入 primary 节点
	for _, nodeID := range primary {
		seen[nodeID] = struct{}{}
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
	// 双写：然后写入 hinted 节点
	for _, nodeID := range hinted {
		if _, ok := seen[nodeID]; ok {
			continue
		}
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
	primary, fallback, err := s.memberBridge.ResolveReadOwners(key)
	if err != nil {
		return "", err
	}
	// 尝试从负责该 key 的节点中读取，读取到任意一个节点的数据即可返回
	seen := make(map[string]struct{})
	// 优先从 primary 节点读取
	for _, nodeID := range primary {
		seen[nodeID] = struct{}{}
		if nodeID == s.memberBridge.SelfID() {
			if value, ok, err := s.st.Get(ctx, key); err == nil && ok {
				return value, nil
			} else if err != nil {
				return "", err
			} else {
				return "", errors.Error{Type: errors.ObjectNotFound, Info: "key not found"}
			}
		} else {
			value, err := s.memberBridge.ForwardGet(ctx, nodeID, key)
			if err == nil {
				return value, nil
			}
		}
	}
	// primary 读取失败则尝试从 hinted 节点读取
	for _, nodeID := range fallback {
		if _, ok := seen[nodeID]; ok {
			continue
		}
		if nodeID == s.memberBridge.SelfID() {
			if value, ok, err := s.st.Get(ctx, key); err == nil && ok {
				return value, nil
			} else if err != nil {
				return "", err
			} else {
				return "", errors.Error{Type: errors.ObjectNotFound, Info: "key not found"}
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
	primary, hinted, err := s.memberBridge.ResolveWriteOwners(key)
	// 将请求发送到所有负责该 key 的节点
	if err != nil {
		return err
	}
	seen := make(map[string]struct{})
	// 优先删除 primary 节点
	for _, nodeID := range primary {
		seen[nodeID] = struct{}{}
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
	// 然后删除 hinted 节点
	for _, nodeID := range hinted {
		if _, ok := seen[nodeID]; ok {
			continue
		}
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
