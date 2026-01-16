package bridge

import (
	"context"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
)

// 返回指定节点对外服务地址
func (b *MemberBridge) ClientAddress(nodeID string) (addr string, ok bool) {
	if b == nil {
		return "", false
	}
	v, exists := b.memberAddrs.Load(nodeID)
	if !exists {
		return "", false
	}
	info, typeOK := v.(MemberAddrInfo)
	if !typeOK {
		return "", false
	}
	if info.clientAddress == "" {
		return "", false
	}
	return info.clientAddress, true
}

// 返回指定节点 chash 内部通信地址
func (b *MemberBridge) CHashGRPCAddress(nodeID string) (addr string, ok bool) {
	if b == nil {
		return "", false
	}
	v, exists := b.memberAddrs.Load(nodeID)
	if !exists {
		return "", false
	}
	info, typeOK := v.(MemberAddrInfo)
	if !typeOK {
		return "", false
	}
	if info.chashGRPCAddress == "" {
		return "", false
	}
	return info.chashGRPCAddress, true
}

// region 远程业务请求转发相关

// 将 Put 业务请求转发到目标节点
func (b *MemberBridge) ForwardPut(ctx context.Context, nodeID, key, value string) error {
	if b == nil || b.remoteClient == nil {
		return errors.Error{Type: errors.ImportError, Info: "remote client not initialized"}
	}
	addr, ok := b.ClientAddress(nodeID)
	if !ok {
		return errors.Error{Type: errors.ImportError, Info: "client address not found"}
	}
	return b.remoteClient.Put(ctx, addr, key, value)
}

// 将 Get 业务请求转发到目标节点
func (b *MemberBridge) ForwardGet(ctx context.Context, nodeID, key string) (string, error) {
	if b == nil || b.remoteClient == nil {
		return "", errors.Error{Type: errors.ImportError, Info: "remote client not initialized"}
	}
	addr, ok := b.ClientAddress(nodeID)
	if !ok {
		return "", errors.Error{Type: errors.ImportError, Info: "client address not found"}
	}
	return b.remoteClient.Get(ctx, addr, key)
}

// 将 Delete 业务请求转发到目标节点
func (b *MemberBridge) ForwardDelete(ctx context.Context, nodeID, key string) error {
	if b == nil || b.remoteClient == nil {
		return errors.Error{Type: errors.ImportError, Info: "remote client not initialized"}
	}
	addr, ok := b.ClientAddress(nodeID)
	if !ok {
		return errors.Error{Type: errors.ImportError, Info: "client address not found"}
	}
	return b.remoteClient.Delete(ctx, addr, key)
}

// endregion

// region 内部通信方法

// 内部通信：向目标节点推送一批 KVPair 数据
func (b *MemberBridge) PushBatch(ctx context.Context, moveID uint32, nodeID string, startHash, endHash uint32) error {
	if b == nil || b.transport == nil {
		return errors.Error{Type: errors.ImportError, Info: "transport not initialized"}
	}
	kvs, err := b.st.GetHashRange(ctx, startHash, endHash)
	if err != nil {
		return err
	}
	addr, ok := b.CHashGRPCAddress(nodeID)
	if !ok {
		return errors.Error{Type: errors.ImportError, Info: "client address not found"}
	}
	return b.transport.PushBatch(ctx, moveID, addr, kvs)
}

// 内部通信：从目标节点拉取 Key 哈希索引 [startHash, endHash) 的 KVPair 列表
func (b *MemberBridge) PullRange(ctx context.Context, moveID uint32, nodeID string, startHash, endHash uint32) error {
	if b == nil || b.transport == nil {
		return errors.Error{Type: errors.ImportError, Info: "transport not initialized"}
	}
	addr, ok := b.CHashGRPCAddress(nodeID)
	if !ok {
		return errors.Error{Type: errors.ImportError, Info: "client address not found"}
	}
	kvs, err := b.transport.PullRange(ctx, moveID, addr, startHash, endHash)
	if err != nil {
		return err
	}
	_, err = b.st.AppendBatchKV(ctx, kvs)
	return err
}

// 内部通信：向目标节点复制一批 Command（语义上可用于副本写/反熵）
func (b *MemberBridge) Replicate(ctx context.Context, nodeID string, cmds *[]common.Command) error {
	if b == nil || b.transport == nil {
		return errors.Error{Type: errors.ImportError, Info: "transport not initialized"}
	}
	addr, ok := b.CHashGRPCAddress(nodeID)
	if !ok {
		return errors.Error{Type: errors.ImportError, Info: "client address not found"}
	}
	return b.transport.Replicate(ctx, addr, cmds)
}

// 预告迁移计划：向目标节点广播迁移计划，返回节点总数和成功预告数
func (b *MemberBridge) AnnouncePlan(ctx context.Context, hints *[]chash.MovePlanHint) (int, int, error) {
	if b == nil || b.transport == nil {
		return 0, 0, errors.Error{Type: errors.ImportError, Info: "transport not initialized"}
	}
	selfId := b.gossipNode.SelfID()
	announced := 0
	// 从计划中收集所有涉及的节点，跳过自己
	nodeSet := make(map[string]struct{})
	for _, hint := range *hints {
		for _, nodeID := range hint.OldOwners {
			if nodeID == selfId {
				continue
			}
			nodeSet[nodeID] = struct{}{}
		}
		for _, nodeID := range hint.NewOwners {
			if nodeID == selfId {
				continue
			}
			nodeSet[nodeID] = struct{}{}
		}
	}
	// 逐节点发送
	for nodeID := range nodeSet {
		addr, ok := b.CHashGRPCAddress(nodeID)
		if !ok {
			continue
		}
		err := b.transport.AnnouncePlan(ctx, addr, hints)
		if err != nil {
			continue
		}
		announced++
	}
	return len(nodeSet), announced, nil
}

// 拉取计划：从目标节点同步指定 epoch 之后的计划
func (b *MemberBridge) PullPlanSince(ctx context.Context, nodeID string, sinceEpoch uint64) (*[]chash.MovePlanHint, error) {
	if b == nil || b.transport == nil {
		return nil, errors.Error{Type: errors.ImportError, Info: "transport not initialized"}
	}
	addr, ok := b.CHashGRPCAddress(nodeID)
	if !ok {
		return nil, errors.Error{Type: errors.ImportError, Info: "client address not found"}
	}
	return b.transport.PullPlanSince(ctx, addr, sinceEpoch)
}

// endregion
