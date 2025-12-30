package chash

import (
	"distributed-kv-store/internal/errors"
	"fmt"
	"hash/crc32"
	"slices"
	"sort"
	"strconv"
)

func hashKey(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}

// 获取给定键对应的节点 ID（顺时针最近的虚拟节点所属物理节点）
func (r *HashRing) GetNode(key string) (nodeID string, ok bool, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ringKeys) == 0 {
		return "", false, nil
	}

	h := hashKey(key)
	idx := sort.Search(len(r.ringKeys), func(i int) bool {
		return r.ringKeys[i] >= h
	})
	if idx == len(r.ringKeys) {
		idx = 0 // 环绕回到第一个节点
	}
	owner, exists := r.vnodeOwners[r.ringKeys[idx]]
	if !exists {
		// 理论上不会发生
		return "", false, errors.ErrNoVNodeOwner
	}
	return owner, true, nil
}

// 获取给定键对应的多个节点 ID（用于副本等场景）
func (r *HashRing) GetNodes(key string) (nodeIDs []string, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.ringKeys) == 0 {
		return nil, nil
	}

	h := hashKey(key)
	idx := sort.Search(len(r.ringKeys), func(i int) bool {
		return r.ringKeys[i] >= h
	})
	visited := make(map[string]struct{})
	for len(visited) < len(r.nodes) {
		if idx == len(r.ringKeys) {
			idx = 0 // 环绕回到第一个节点
		}
		owner, exists := r.vnodeOwners[r.ringKeys[idx]]
		if !exists {
			return nil, errors.ErrNoVNodeOwner
		}
		// 避免重复添加相同节点
		if _, seen := visited[owner]; !seen {
			visited[owner] = struct{}{}
			nodeIDs = append(nodeIDs, owner)
		}
		idx++
	}
	return nodeIDs, nil
}

// 添加节点并重建环
func (r *HashRing) AddNode(node Node) error {
	// 可能涉及数据迁移，留待后续实现
	// 节点间通信需要外部调用 Transport 层完成
	return nil
}

// 移除节点并重建环
func (r *HashRing) RemoveNode(nodeID string) error {
	// 可能涉及数据迁移，留待后续实现
	// 节点间通信需要外部调用 Transport 层完成
	return nil
}

// 全量重建环
func (r *HashRing) Rebuild(nodes []Node) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// reset
	r.nodes = append([]Node(nil), nodes...)
	r.ringKeys = r.ringKeys[:0]
	r.vnodeOwners = make(map[uint32]string)
	r.VitrualNodesMap = make(map[string]string)

	virtualNodes := r.VirtualNodes
	if virtualNodes <= 0 {
		virtualNodes = 1
	}

	for _, n := range nodes {
		weight := n.weight
		if weight <= 0 {
			weight = 1
		}
		replicas := virtualNodes * weight // 根据节点权重调整虚拟节点数
		for i := range replicas {
			// 用 nodeID + replicaIndex 生成虚拟节点 key
			vkey := fmt.Sprintf("%s#%d", n.id, i)
			h := hashKey(vkey)
			r.vnodeOwners[h] = n.id
			r.VitrualNodesMap[strconv.FormatUint(uint64(h), 10)] = n.id
			r.ringKeys = append(r.ringKeys, h)
		}
	}

	slices.Sort(r.ringKeys)
	return nil
}
