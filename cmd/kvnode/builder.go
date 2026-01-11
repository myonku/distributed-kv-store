package main

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/bridge"
	"distributed-kv-store/internal/chash"
	"distributed-kv-store/internal/chash/chash_grpc"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/gossip"
	"distributed-kv-store/internal/gossip/gossip_grpc"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/raft/raft_grpc"
	"distributed-kv-store/internal/raft/raft_store"
	"distributed-kv-store/internal/services"
	"distributed-kv-store/internal/storage"

	"google.golang.org/grpc"
)

// 根据配置的运行模式构造对应的 KVService 实现，返回部分引用
func buildKVService(appCfg *configs.AppConfig) (storage.Storage, services.KVService, *raft.Node, *gossip.Node, error) {
	// Storage 统一创建，全局管理
	st, err := storage.NewStorage(appCfg.Storage)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	switch appCfg.Mode {
	case configs.ModeStandalone:
		// 单机模式下直接返回 Storage 和 StandaloneKVService
		st, err := storage.NewStorage(appCfg.Storage)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return st, services.NewStandaloneKVService(st), nil, nil, nil
	case configs.ModeRaft:
		// Raft 模式下构造 RaftKVService
		remoteClient := common.NewCommonRemoteClient()
		kvService, node, err := buildRaftKVService(appCfg, st, remoteClient)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return st, kvService, node, nil, nil
	case configs.ModeConsHashGossip:
		// 一致性哈希 + Gossip 模式下构造 CHashKVService
		remoteClient := common.NewCommonRemoteClient()
		kvService, node, err := buildConsHashGossipKVService(appCfg, st, remoteClient)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return st, kvService, nil, node, nil
	default:
		return nil, nil, nil, nil, errors.ErrUnSupportedMode
	}
}

// Raft 模式下构造 RaftKVService，返回 Raft 节点引用
func buildRaftKVService(
	appCfg *configs.AppConfig,
	st storage.Storage,
	remoteClient common.RemoteClient,
) (services.KVService, *raft.Node, error) {
	sm := &raft_store.KVStateMachine{St: st}    // 状态机
	logStore := raft_store.NewRaftLogStore(st)  // Raft 日志存储
	hsStore := raft_store.NewHardStateStore(st) // Raft 持久化状态存储
	transport, err := raft_grpc.NewGRPCTransport(appCfg.Membership.Peers)
	if err != nil {
		return nil, nil, errors.ErrCreateTransportFailed
	}
	// 创建 Raft 节点
	raftNode := raft.NewNode(appCfg, sm, logStore, hsStore, transport)
	return services.NewRaftKVService(st, raftNode, remoteClient), raftNode, nil
}

// 一致性哈希 + Gossip 模式下构造 CHashKVService，返回 Gossip 节点引用
func buildConsHashGossipKVService(
	appCfg *configs.AppConfig,
	st storage.Storage,
	remoteClient common.RemoteClient,
) (services.KVService, *gossip.Node, error) {
	ring := chash.NewHashRing(appCfg)
	gossipTransport, err := gossip_grpc.NewGRPCTransport(appCfg.Membership.Peers)
	if err != nil {
		return nil, nil, errors.ErrCreateTransportFailed
	}
	chashTransport := chash_grpc.NewGRPCTransport()
	gossipNode := gossip.NewNode(appCfg, gossipTransport)
	memberBridge := bridge.NewMemberBridge(
		gossipNode,
		ring,
		chashTransport,
		remoteClient,
		st,
	)
	kvService := services.NewCHashKVService(memberBridge, st)
	return kvService, gossipNode, nil
}

// 创建并注册 gRPC 服务器
func buildGRPCServer(
	appCfg *configs.AppConfig,
	st storage.Storage,
	raftNode *raft.Node,
	gossipNode *gossip.Node,
) ([]*grpc.Server, []string, error) {
	switch appCfg.Mode {
	case configs.ModeRaft:
		// Raft 模式下创建 Raft gRPC 服务器
		srv := raft_grpc.NewRaftGRPCServer(raftNode)
		grpcServer := NewGRPCServer()
		raft_grpc.RegisterRaftServiceServer(grpcServer, srv)
		return []*grpc.Server{grpcServer}, []string{appCfg.Self.RaftGRPCAddress}, nil
	case configs.ModeConsHashGossip:
		// 一致性哈希 + Gossip 模式下创建 Gossip 和 CHash gRPC 服务器
		gossipSrv := gossip_grpc.NewGossipGRPCServer(gossipNode)
		gossipGRPCServer := NewGRPCServer()
		gossip_grpc.RegisterGossipServiceServer(gossipGRPCServer, gossipSrv)
		chashSrv := chash_grpc.NewChashGRPCServer(st)
		chashGRPCServer := NewGRPCServer()
		chash_grpc.RegisterCHashServiceServer(chashGRPCServer, chashSrv)
		return []*grpc.Server{gossipGRPCServer, chashGRPCServer}, []string{appCfg.Self.GossipGRPCAddress, appCfg.Self.CHashGRPCAddress}, nil
	default:
		// 其他模式不需要 gRPC 服务
		return nil, nil, nil
	}
}

// 生成一个标准 grpc.Server，供外部统一创建
func NewGRPCServer() *grpc.Server {
	return grpc.NewServer()
}
