package chash

import "sort"

// RecordPlanHints 将迁移提示缓存到 ring
func (r *HashRing) RecordPlanHints(hints *[]MovePlanHint) {
	if r == nil || len(*hints) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, hint := range *hints {
		updated := false
		for i := range r.planHints {
			if r.planHints[i].Epoch == hint.Epoch &&
				r.planHints[i].StartHash == hint.StartHash &&
				r.planHints[i].EndHash == hint.EndHash {

				r.planHints[i] = hint
				updated = true
				break
			}
		}
		if !updated {
			r.planHints = append(r.planHints, hint)
		}
	}
}

// PlanHintsSince 返回 sinceEpoch 之后的提示（按 epoch 升序）
func (r *HashRing) PlanHintsSince(sinceEpoch uint64) *[]MovePlanHint {
	if r == nil {
		return &[]MovePlanHint{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MovePlanHint, 0)
	for _, h := range r.planHints {
		if h.Epoch > sinceEpoch {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Epoch < out[j].Epoch })
	return &out
}

// LookupPlanHintForHash 根据 hash 查找最新匹配的迁移提示
func (r *HashRing) LookupPlanHintForHash(hash uint32) (MovePlanHint, bool) {
	if r == nil {
		return MovePlanHint{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var found MovePlanHint
	ok := false
	// 查找所有匹配的提示，返回 epoch 最大的那个
	for _, h := range r.planHints {
		if containsHash(h.StartHash, h.EndHash, hash) {
			if !ok || h.Epoch > found.Epoch {
				found = h
				ok = true
			}
		}
	}
	return found, ok
}

// UpdatePlanHintStatus 更新指定范围提示的状态
func (r *HashRing) UpdatePlanHintStatus(epoch uint64, startHash, endHash uint32, status MigrationStatus) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.planHints {
		if r.planHints[i].Epoch == epoch &&
			r.planHints[i].StartHash == startHash &&
			r.planHints[i].EndHash == endHash {
			r.planHints[i].Status = status
			return
		}
	}
}

// ClearPlanHintsBefore 清理指定 epoch 之前的提示
func (r *HashRing) ClearPlanHintsBefore(epoch uint64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.planHints[:0]
	for _, h := range r.planHints {
		if h.Epoch >= epoch {
			kept = append(kept, h)
		}
	}
	r.planHints = kept
}

// 判断 hash 是否在指定范围内
func containsHash(startHash, endHash, hash uint32) bool {
	if startHash == endHash {
		return false
	}
	if startHash < endHash {
		return hash >= startHash && hash < endHash
	}
	return hash >= startHash || hash < endHash
}
