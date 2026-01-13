package bridge

import (
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/gossip"
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
	return nil
}
