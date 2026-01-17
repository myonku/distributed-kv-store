package bridge

import (
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/gossip"
	"fmt"
)

// 事件循环：消费 gossip event，更新环状态并投递平衡计划
func (b *MemberBridge) runEventLoop() {
	defer b.wg.Done()

	if b == nil || b.gossipNode == nil || b.consHashRing == nil {
		return
	}

	// 默认 MemberBridge 是唯一事件消费者，所有事件需通过该桥接器处理
	// 后续可能会有其他消费者，则需要改为广播模式
	events := b.gossipNode.Events()

	for {
		select {
		case <-b.ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			// 事件循环只负责投递最新快照
			b.enqueueSnapshot(ev.Snapshot)
		}
	}
}

// 事件循环：消费 Ring 平衡计划并执行数据迁移
func (b *MemberBridge) runBalanceLoop() {
	defer b.wg.Done()

	if b == nil || b.consHashRing == nil {
		return
	}

	for {
		select {
		case <-b.ctx.Done():
			return
		case snapshot, ok := <-b.snapshotCh:
			if !ok {
				return
			}

			plan, err := b.rebuildFromSnapshot(snapshot)
			if err != nil {
				continue
			}

			// 执行迁移前先通知其他节点
			_, _, _ = b.AnnouncePlan(b.ctx, &plan.Hints)

			// 记录计划提示
			b.RecordPlanHints(plan)
			// 跟踪计划中的 move 任务
			b.trackPlanMoves(plan)

			// epoch 未更新则无需处理
			if plan.Epoch <= b.lastAppliedEpoch {
				continue
			}
			b.lastAppliedEpoch = plan.Epoch

			for _, move := range plan.Moves {
				moveCopy := move
				epochCopy := plan.Epoch
				go b.excuteMovePlan(epochCopy, moveCopy)
			}
		}
	}
}

// 将最新快照投递给 balance loop。
// 采用“覆盖式”投递策略：当通道已满时，丢弃旧快照，保证尽快应用最新视图
func (b *MemberBridge) enqueueSnapshot(snapshot []gossip.Member) {
	if b == nil {
		return
	}
	// 拷贝一份，避免调用方后续修改底层 slice
	ss := make([]gossip.Member, len(snapshot))
	copy(ss, snapshot)

	select {
	case b.snapshotCh <- ss:
		return
	default:
		// 丢弃旧快照
		select {
		case <-b.snapshotCh:
		default:
		}
		select {
		case b.snapshotCh <- ss:
		default:
		}
	}
}

// 执行单个数据迁移计划
func (m *MemberBridge) excuteMovePlan(epoch uint64, move chash.MoveRange) error {
	if m == nil || m.transport == nil || m.st == nil {
		return errors.Error{Type: errors.ImportError, Info: "member bridge not initialized"}
	}
	selfID := m.gossipNode.SelfID()
	// 如果 双方节点都不是本节点 则跳过
	if move.FromID != selfID && move.ToNodeID != selfID {
		return nil
	}

	// 标记该范围进入迁移中（具体语义可后续细化）
	m.UpdatePlanHintStatus(
		epoch,
		move.StartHash,
		move.EndHash,
		chash.MigrationStatusInProgress,
	)

	// 计算 moveID（用于去重）
	moveID := common.ComputeMoveID(
		epoch,
		move.StartHash,
		move.EndHash,
		move.FromID,
		move.ToNodeID,
	)

	// 已完成则跳过
	exists, err := m.st.GetMoveRangeRecord(m.ctx, moveID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	// From 是自己，To 是远端节点，推送数据
	if move.FromID == selfID {
		err := m.PushBatch(
			m.ctx,
			moveID,
			move.ToNodeID,
			move.StartHash,
			move.EndHash,
		)
		if err != nil {
			return err
		}
	}
	// To 是自己，From 是远端节点，拉取数据
	if move.ToNodeID == selfID {
		err := m.PullRange(
			m.ctx,
			moveID,
			move.FromID,
			move.StartHash,
			move.EndHash,
		)
		if err != nil {
			return err
		}
	}

	// 仅在成功完成 push/pull 后记录，避免失败后永远跳过
	if err := m.st.SaveMoveRangeRecord(m.ctx, moveID); err != nil {
		return err
	}

	// 标记该范围完成一次迁移，若该范围所有迁移完成则更新为 completed
	if m.markMoveDone(epoch, move.StartHash, move.EndHash) {
		m.UpdatePlanHintStatus(
			epoch,
			move.StartHash,
			move.EndHash,
			chash.MigrationStatusCompleted,
		)
	}
	return nil
}

// 跟踪迁移计划中的 move 任务数量
func (b *MemberBridge) trackPlanMoves(plan chash.MovePlan) {
	if b == nil || len(plan.Moves) == 0 {
		return
	}
	selfID := b.SelfID()
	b.planMu.Lock()
	defer b.planMu.Unlock()
	for _, mv := range plan.Moves {
		if mv.FromID != selfID && mv.ToNodeID != selfID {
			continue
		}
		key := rangeKey(plan.Epoch, mv.StartHash, mv.EndHash)
		b.moveRangeTotal[key]++
	}
}

// 标记某个范围的单次迁移完成，返回该范围是否所有迁移均已完成
func (b *MemberBridge) markMoveDone(epoch uint64, startHash, endHash uint32) bool {
	if b == nil {
		return false
	}
	key := rangeKey(epoch, startHash, endHash)
	b.planMu.Lock()
	defer b.planMu.Unlock()
	if _, ok := b.moveRangeTotal[key]; !ok {
		return false
	}
	b.moveRangeDone[key]++
	if b.moveRangeDone[key] >= b.moveRangeTotal[key] {
		delete(b.moveRangeTotal, key)
		delete(b.moveRangeDone, key)
		return true
	}
	return false
}

func rangeKey(epoch uint64, startHash, endHash uint32) string {
	return fmt.Sprintf("%d:%d:%d", epoch, startHash, endHash)
}
