package chash

import (
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"fmt"
	"maps"
	"slices"
	"sort"
)

// 用于数据再平衡的计划结构体
type RebalancePlan struct {
	Epoch    uint64      // 计划所属的 epoch
	Moves    []MoveRange // 节点ID->数据迁移范围映射
	CopyOnly bool        // 是否仅执行数据复制（不删除旧数据）
}

type MoveRange struct {
	FromID    string // 源节点 ID
	ToNodeID  string // 目标节点 ID
	StartHash uint32 // 起始哈希值（包含）
	EndHash   uint32 // 结束哈希值（不包含）
}

// 重建环并返回数据迁移计划
func (r *HashRing) RebuildWithPlan(nodes []Node) (plan RebalancePlan, err error) {
	if r == nil {
		return RebalancePlan{}, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 判断 ring 是否真正变化
	if SameRingLocked(r, nodes) {
		return RebalancePlan{Epoch: r.epoch}, nil
	}

	// 备份旧 ring 状态
	oldKeys := slices.Clone(r.ringKeys)
	oldOwners := make(map[uint32]string, len(r.vnodeOwners))
	maps.Copy(oldOwners, r.vnodeOwners)

	// 构建新 ring 状态
	newKeys, newOwners, newVNodeMap := BuildRingState(nodes, r.VirtualNodes)

	// 基于 old/new 生成迁移计划（默认副本场景）
	// 对每个区间 [start,end)，比较 old/new 的副本集合：
	// - newSet \ oldSet：需要新增的副本（从 oldSet 的任一现存副本复制，默认取 oldSet[0]）
	// - oldSet \ newSet：是否删除旧副本不在本阶段处理（copy-only）
	planMoves := make([]MoveRange, 0)
	if len(oldKeys) != 0 && len(newKeys) != 0 {
		newRF := r.replicationFactor
		if newRF <= 0 {
			newRF = 1
		}
		if newRF > len(nodes) {
			newRF = len(nodes)
		}

		oldRF := r.replicationFactor
		if oldRF <= 0 {
			oldRF = 1
		}
		if oldRF > len(r.nodes) {
			oldRF = len(r.nodes)
		}

		for i, cur := range newKeys {
			prev := newKeys[len(newKeys)-1]
			if i > 0 {
				prev = newKeys[i-1]
			}
			if prev == cur {
				continue
			}
			start := prev + 1
			end := cur + 1

			// 确保新 ring 对该 vnode 有 owner
			if _, ok := newOwners[cur]; !ok {
				return RebalancePlan{}, errors.ErrNoFoundNewOwner
			}

			newSet := LookupOwners(newKeys, newOwners, start, newRF)
			oldSet := LookupOwners(oldKeys, oldOwners, start, oldRF)
			if len(newSet) == 0 || len(oldSet) == 0 {
				continue
			}

			oldSetMap := make(map[string]struct{}, len(oldSet))
			for _, id := range oldSet {
				oldSetMap[id] = struct{}{}
			}

			src := oldSet[0]
			for _, dst := range newSet {
				if dst == "" || dst == src {
					continue
				}
				if _, ok := oldSetMap[dst]; ok {
					continue
				}
				planMoves = append(planMoves, MoveRange{
					FromID:    src,
					ToNodeID:  dst,
					StartHash: start,
					EndHash:   end,
				})
			}
		}
		planMoves = mergeAdjacentMoves(planMoves)
	}

	// 应用新 ring 并 bump epoch
	r.nodes = append([]Node(nil), nodes...)
	r.ringKeys = newKeys
	r.vnodeOwners = newOwners
	r.VitrualNodesMap = newVNodeMap
	r.epoch++

	return RebalancePlan{Epoch: r.epoch, CopyOnly: true, Moves: planMoves}, nil
}

// 合并相邻的迁移范围以减少计划条目数
func mergeAdjacentMoves(moves []MoveRange) []MoveRange {
	if len(moves) <= 1 {
		return moves
	}
	out := make([]MoveRange, 0, len(moves))
	for _, mv := range moves {
		if len(out) == 0 {
			out = append(out, mv)
			continue
		}
		last := &out[len(out)-1]
		if last.FromID == mv.FromID && last.ToNodeID == mv.ToNodeID && last.EndHash == mv.StartHash {
			last.EndHash = mv.EndHash
			continue
		}
		out = append(out, mv)
	}
	return out
}

// 判断两个节点列表是否表示相同的环状态（节点ID + 权重 + VirtualNodes）
func SameRingLocked(r *HashRing, nodes []Node) bool {
	if r == nil {
		return false
	}
	if len(r.nodes) != len(nodes) {
		return false
	}
	// VirtualNodes 变化也会改变 vnode 数量（本程序内可以认为不会动态变更虚拟节点数量）
	// 此函数的上下文没有 VirtualNodes 的相关参数，因此只比较节点 ID 和权重
	existing := make(map[string]int, len(r.nodes))
	for _, n := range r.nodes {
		existing[n.id] = n.weight
	}
	for _, n := range nodes {
		w, ok := existing[n.id]
		if !ok || w != n.weight {
			return false
		}
	}
	return true
}

// 构建环状态（hash 列表 + vnode->node 映射 + vnode map）
func BuildRingState(nodes []Node, virtualNodes int) ([]uint32, map[uint32]string, map[string]string) {
	if virtualNodes <= 0 {
		virtualNodes = 1
	}
	keys := make([]uint32, 0)
	owners := make(map[uint32]string)
	vnodeMap := make(map[string]string)

	for _, n := range nodes {
		weight := n.weight
		if weight <= 0 {
			weight = 1
		}
		replicas := virtualNodes * weight
		for i := range replicas {
			vkey := fmt.Sprintf("%s#%d", n.id, i)
			h := common.HashKey(vkey)
			owners[h] = n.id
			vnodeMap[fmt.Sprintf("%d", h)] = n.id
			keys = append(keys, h)
		}
	}
	slices.Sort(keys)
	return keys, owners, vnodeMap
}

// 查找给定哈希值的拥有者节点 ID (副本场景)
func LookupOwners(keys []uint32, owners map[uint32]string, h uint32, replicationFactor int) (nodeIDs []string) {
	if len(keys) == 0 {
		return []string{}
	}
	if replicationFactor <= 0 {
		replicationFactor = 1
	}
	idx := sort.Search(len(keys), func(i int) bool {
		return keys[i] >= h
	})
	seen := make(map[string]struct{})
	// 收集不同的节点 ID，直到达到副本因子要求；
	// 为避免 replicationFactor > 物理节点数导致死循环，这里最多扫描一圈 keys
	steps := 0
	for len(seen) < replicationFactor && steps < len(keys) {
		if idx == len(keys) {
			idx = 0 // 环绕回到第一个节点
		}
		owner, exists := owners[keys[idx]]
		if !exists {
			// 理论上不会发生
			return nil
		}
		if _, ok := seen[owner]; !ok {
			seen[owner] = struct{}{}
			nodeIDs = append(nodeIDs, owner)
		}
		idx++
		steps++
	}
	return nodeIDs
}
