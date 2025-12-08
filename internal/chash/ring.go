package chash

import (
	"hash/crc32"
	"slices"
	"sort"
	"strconv"
	"sync"
)

// 定义一致性哈希环的抽象接口
type Ring interface {
	AddNode(nodeID string, weight int)
	RemoveNode(nodeID string)
	GetNode(key string) (nodeID string, ok bool)
}

// 基于虚拟节点的简单一致性哈希实现
type hashRing struct {
	mu sync.RWMutex

	// 有序的虚拟节点位置（哈希值）
	keys []uint32
	// 位置 -> 真实节点 ID
	ring map[uint32]string
}

// 创建一个空的 hashRing
func NewHashRing() Ring {
	return &hashRing{
		keys: make([]uint32, 0),
		ring: make(map[uint32]string),
	}
}

func (h *hashRing) AddNode(nodeID string, weight int) {
	if weight <= 0 {
		weight = 1
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for i := 0; i < weight; i++ {
		// 虚拟节点 key: nodeID#i
		vkey := nodeID + "#" + strconv.Itoa(i)
		hv := crc32.ChecksumIEEE([]byte(vkey))
		if _, exists := h.ring[hv]; exists {
			continue
		}
		h.ring[hv] = nodeID
		h.keys = append(h.keys, hv)
	}

	slices.Sort(h.keys)
}

func (h *hashRing) RemoveNode(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.keys) == 0 {
		return
	}

	newKeys := h.keys[:0]
	for _, k := range h.keys {
		if h.ring[k] == nodeID {
			delete(h.ring, k)
			continue
		}
		newKeys = append(newKeys, k)
	}
	h.keys = newKeys
}

func (h *hashRing) GetNode(key string) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.keys) == 0 {
		return "", false
	}

	hv := crc32.ChecksumIEEE([]byte(key))
	// 在有序切片中找到第一个 >= hv 的位置
	idx := sort.Search(len(h.keys), func(i int) bool { return h.keys[i] >= hv })
	if idx == len(h.keys) {
		idx = 0 // 回环
	}
	nodeID, ok := h.ring[h.keys[idx]]
	return nodeID, ok
}
