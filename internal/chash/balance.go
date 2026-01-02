package chash

// 用于数据再平衡的计划结构体
type RebalancePlan struct {
	Epoch uint64               // 计划所属的 epoch
	Moves map[string]MoveRange // 节点ID->数据迁移范围映射
}

type MoveRange struct {
	fromID    string // 源节点 ID
	toNodeID  string // 目标节点 ID
	startHash uint32 // 起始哈希值（包含）
	endHash   uint32 // 结束哈希值（不包含）
}

// 重建环并返回数据迁移计划
func (r *HashRing) RebuildWithPlan(nodes []Node) (plan RebalancePlan, err error) {

	r.mu.RLock()
	same := len(r.nodes) == len(nodes)
	r.mu.RUnlock()

	// 对比现有环和新环，如果没有变化则不进行重建
	if same {
		r.mu.RLock()
		existingNodes := make(map[string]Node)
		for _, n := range r.nodes {
			existingNodes[n.id] = n
		}
		r.mu.RUnlock()
		for _, n := range nodes {
			if _, exists := existingNodes[n.id]; !exists {
				same = false
				break
			}
		}
	}
	// 环没有变化，无需重建
	if same {
		return RebalancePlan{}, nil
	}

	// 环有变化，执行重建

	return RebalancePlan{}, nil
}
