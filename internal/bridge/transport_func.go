package bridge

import (
	"context"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/storage"
)

// 根据 key 查询负责节点 ID
func (b *MemberBridge) OwnerNodeID(key string) (nodeID string, ok bool, err error) {
	if b == nil || b.consHashRing == nil {
		return "", false, errors.ErrResourceNotInit
	}
	return b.consHashRing.GetNode(key)
}

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
		return errors.ErrResourceNotInit
	}
	addr, ok := b.ClientAddress(nodeID)
	if !ok {
		return errors.ErrResourceNotInit
	}
	return b.remoteClient.Put(ctx, addr, key, value)
}

// 将 Get 业务请求转发到目标节点
func (b *MemberBridge) ForwardGet(ctx context.Context, nodeID, key string) (string, error) {
	if b == nil || b.remoteClient == nil {
		return "", errors.ErrResourceNotInit
	}
	addr, ok := b.ClientAddress(nodeID)
	if !ok {
		return "", errors.ErrResourceNotInit
	}
	return b.remoteClient.Get(ctx, addr, key)
}

// 将 Delete 业务请求转发到目标节点
func (b *MemberBridge) ForwardDelete(ctx context.Context, nodeID, key string) error {
	if b == nil || b.remoteClient == nil {
		return errors.ErrResourceNotInit
	}
	addr, ok := b.ClientAddress(nodeID)
	if !ok {
		return errors.ErrResourceNotInit
	}
	return b.remoteClient.Delete(ctx, addr, key)
}

// endregion

// region 内部通信方法

// 内部通信：向目标节点推送一批 Command
func (b *MemberBridge) PushBatch(ctx context.Context, nodeID string, cmds *[]storage.Command) error {
	if b == nil || b.transport == nil {
		return errors.ErrResourceNotInit
	}
	addr, ok := b.CHashGRPCAddress(nodeID)
	if !ok {
		return errors.ErrResourceNotInit
	}
	return b.transport.PushBatch(ctx, addr, cmds)
}

// 内部通信：从目标节点拉取日志索引 [startIndex, endIndex) 的 Command 列表
func (b *MemberBridge) PullRange(ctx context.Context, nodeID string, startIndex, endIndex uint64) (*[]storage.Command, error) {
	if b == nil || b.transport == nil {
		return nil, errors.ErrResourceNotInit
	}
	addr, ok := b.CHashGRPCAddress(nodeID)
	if !ok {
		return nil, errors.ErrResourceNotInit
	}
	return b.transport.PullRange(ctx, addr, startIndex, endIndex)
}

// 内部通信：向目标节点复制一批 Command（语义上可用于副本写/反熵）
func (b *MemberBridge) Replicate(ctx context.Context, nodeID string, cmds *[]storage.Command) error {
	if b == nil || b.transport == nil {
		return errors.ErrResourceNotInit
	}
	addr, ok := b.CHashGRPCAddress(nodeID)
	if !ok {
		return errors.ErrResourceNotInit
	}
	return b.transport.Replicate(ctx, addr, cmds)
}

// endregion
