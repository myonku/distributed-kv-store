package chash

import (
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"sort"
)

// 获取给定键对应的节点 ID（顺时针最近的虚拟节点所属物理节点）
func (r *HashRing) GetNode(key string) (nodeID string, ok bool, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ringKeys) == 0 {
		return "", false, nil
	}

	h := common.HashKey(key)
	idx := sort.Search(len(r.ringKeys), func(i int) bool {
		return r.ringKeys[i] >= h
	})
	if idx == len(r.ringKeys) {
		idx = 0 // 环绕回到第一个节点
	}
	owner, exists := r.vnodeOwners[r.ringKeys[idx]]
	if !exists {
		// 理论上不会发生
		return "", false, errors.Error{Type: errors.ObjectNotFound, Info: "vnode owner not found"}
	}
	return owner, true, nil
}

// 获取给定键对应的多个节点 ID（用于副本等场景）
func (r *HashRing) GetNodes(key string) (nodeIDs []string, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	replicationFactor := r.replicationFactor
	if replicationFactor <= 0 {
		replicationFactor = 1
	}
	if replicationFactor > len(r.nodes) {
		replicationFactor = len(r.nodes)
	}
	if len(r.ringKeys) == 0 {
		return []string{}, nil
	}
	h := common.HashKey(key)
	idx := sort.Search(len(r.ringKeys), func(i int) bool {
		return r.ringKeys[i] >= h
	})
	seen := make(map[string]struct{})
	// 收集不同的节点 ID，直到达到副本因子要求；最多扫描一圈 keys，避免死循环。
	steps := 0
	for len(seen) < replicationFactor && steps < len(r.ringKeys) {
		if idx == len(r.ringKeys) {
			idx = 0 // 环绕回到第一个节点
		}
		owner, exists := r.vnodeOwners[r.ringKeys[idx]]
		if !exists {
			// 理论上不会发生
			return nil, errors.Error{Type: errors.ObjectNotFound, Info: "vnode owner not found"}
		}
		if _, ok := seen[owner]; !ok {
			seen[owner] = struct{}{}
			nodeIDs = append(nodeIDs, owner)
		}
		idx++
		steps++
	}
	return nodeIDs, nil
}

// 获取当前环的 epoch 版本
func (r *HashRing) Epoch() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.epoch
}
