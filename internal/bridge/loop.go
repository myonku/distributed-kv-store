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

			// 可能影响 ring 的事件都触发一次全量重建。后续可能根据事件类型做增量 Add/Remove
			switch ev.Type {
			case gossip.EventMemberUp,
				gossip.EventMemberDead,
				gossip.EventMemberSuspect,
				gossip.EventMembershipChanged:
				plan, _ := b.rebuildFromSnapshot(ev.Snapshot)
				// 投递重建计划
				select {
				case b.balancePlanCh <- plan:
				default:
				}
			default:
				// 未知事件：保守起见也重建
				plan, _ := b.rebuildFromSnapshot(ev.Snapshot)
				// 投递重建计划
				select {
				case b.balancePlanCh <- plan:
				default:
				}
			}
		}
	}
}

// 事件循环：消费 Ring 平衡计划并执行数据迁移
func (b *MemberBridge) runBalanceLoop() {
	defer b.wg.Done()

	if b == nil || b.consHashRing == nil || b.remoteClient == nil {
		return
	}

	for {
		select {
		case <-b.ctx.Done():
			return
		case plan, ok := <-b.balancePlanCh:
			if !ok {
				return
			}
			// 如果 plan 为空或 epoch 未更新则跳过
			if plan.Epoch <= b.lastAppliedEpoch || len(plan.Moves) == 0 {
				continue
			}
			// 更新已应用的 epoch
			b.lastAppliedEpoch = plan.Epoch

			// 执行数据迁移
			for _, move := range plan.Moves {
				go b.ExcuteMovePlan(move)
			}
		}
	}
}

// 执行单个数据迁移计划
func (m *MemberBridge) ExcuteMovePlan(move chash.MoveRange) error {
	if m == nil || m.transport == nil {
		return errors.ErrResourceNotInit
	}
	selfID := m.gossipNode.SelfID()
	// 如果 双方节点都不是本节点 则跳过
	if move.FromID != selfID && move.ToNodeID != selfID {
		return nil
	}

	// 计算 moveID（用于去重）
	moveID := common.ComputeMoveID(
		m.lastAppliedEpoch,
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
