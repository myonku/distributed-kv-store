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

// 添加节点并重建环
func (r *HashRing) AddNode(node Node) error {
	// 可能涉及数据迁移，留待后续实现
	return nil
}

// 移除节点并重建环
func (r *HashRing) RemoveNode(nodeID string) error {
	// 可能涉及数据迁移，留待后续实现
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
		weight := n.Weight
		if weight <= 0 {
			weight = 1
		}
		replicas := virtualNodes * weight
		for i := range replicas {
			// 用 nodeID + replicaIndex 生成虚拟节点 key
			vkey := fmt.Sprintf("%s#%d", n.ID, i)
			h := hashKey(vkey)
			r.vnodeOwners[h] = n.ID
			r.VitrualNodesMap[strconv.FormatUint(uint64(h), 10)] = n.ID
			r.ringKeys = append(r.ringKeys, h)
		}
	}

	slices.Sort(r.ringKeys)
	return nil
}
