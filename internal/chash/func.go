package chash

import (
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"sort"
)

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
	// 收集不同的节点 ID，直到达到副本因子要求；最多扫描一圈 keys，避免死循环
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

// 用于解析读路径的 owner 列表，返回当前（新）owners 及更新完成前的兜底（旧）owners
func (r *HashRing) ResolveReadOwners(key string) (primary []string, fallback []string, err error) {
	primary, err = r.GetNodes(key)
	if err != nil {
		return nil, nil, err
	}
	// 查找给定 key 的 hash 值
	h := common.HashKey(key)
	// 查找是否命中某个迁移计划提示
	hint, ok := r.LookupPlanHintForHash(h)
	if !ok {
		// 未命中提示，直接返回 primary 作为唯一 owners
		return primary, []string{}, nil
	}
	// 仅在提示属于当前 epoch 且未完成时才启用兜底
	if hint.Epoch != r.Epoch() || hint.Status == MigrationStatusCompleted {
		return primary, []string{}, nil
	}
	// 命中提示，返回提示中的旧 owners 作为兜底
	seen := make(map[string]struct{}, len(primary))
	for _, id := range primary {
		seen[id] = struct{}{}
	}
	for _, id := range hint.OldOwners {
		if _, exists := seen[id]; exists {
			continue
		}
		fallback = append(fallback, id)
	}
	return primary, fallback, nil
}

// 用于解析写路径的 owner 列表，返回当前（新）owners 及提示写入的（旧）owners
func (r *HashRing) ResolveWriteOwners(key string) (targets []string, hinted []string, err error) {
	targets, err = r.GetNodes(key)
	// 查找给定 key 的 hash 值
	h := common.HashKey(key)
	if err != nil {
		return nil, nil, err
	}
	// 查找是否命中某个迁移计划提示
	hint, ok := r.LookupPlanHintForHash(h)
	if !ok {
		// 未命中提示，直接返回 targets 作为唯一 owners
		return targets, []string{}, nil
	}
	// 仅在提示属于当前 epoch 且未完成时才启用提示写
	if hint.Epoch != r.Epoch() || hint.Status == MigrationStatusCompleted {
		return targets, []string{}, nil
	}
	// 命中提示，返回提示中的旧 owners 作为 hinted
	seen := make(map[string]struct{}, len(targets))
	for _, id := range targets {
		seen[id] = struct{}{}
	}
	for _, id := range hint.OldOwners {
		if _, exists := seen[id]; exists {
			continue
		}
		hinted = append(hinted, id)
	}
	return targets, hinted, nil
}

// 获取当前环的 epoch 版本
func (r *HashRing) Epoch() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.epoch
}
