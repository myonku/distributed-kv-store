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

// 根据配置的运行模式构造对应的 KVService 实现，返回部分引用和清理函数
func buildKVService(
	appCfg *configs.AppConfig,
) (storage.Storage, services.KVService, *raft.Node, *gossip.Node, func(), error) {
	cleanup := func() {}
	// Storage 统一创建，全局管理
	st, err := storage.NewStorage(appCfg.Storage)
	if err != nil {
		return nil, nil, nil, nil, cleanup, err
	}
	switch appCfg.Mode {
	case configs.ModeStandalone:
		// 单机模式下直接返回 Storage 和 StandaloneKVService
		return st, services.NewStandaloneKVService(st), nil, nil, cleanup, nil
	case configs.ModeRaft:
		// Raft 模式下构造 RaftKVService
		remoteClient := common.NewCommonRemoteClient()
		kvService, node, transport, err := buildRaftKVService(appCfg, st, remoteClient)
		if err != nil {
			return nil, nil, nil, nil, cleanup, err
		}
		cleanup = func() {
			if transport != nil {
				_ = transport.Close()
			}
		}
		return st, kvService, node, nil, cleanup, nil
	case configs.ModeConsHashGossip:
		// 一致性哈希 + Gossip 模式下构造 CHashKVService
		remoteClient := common.NewCommonRemoteClient()
		kvService, node, gossipTransport, chashTransport, err := buildConsHashGossipKVService(
			appCfg,
			st,
			remoteClient,
		)
		if err != nil {
			return nil, nil, nil, nil, cleanup, err
		}
		cleanup = func() {
			if gossipTransport != nil {
				_ = gossipTransport.Close()
			}
			if chashTransport != nil {
				_ = chashTransport.Close()
			}
		}
		return st, kvService, nil, node, cleanup, nil
	default:
		return nil, nil, nil, nil, cleanup, errors.Error{Type: errors.InvalidArgument, Info: "unsupported mode"}
	}
}

// 主从模式下构造 RaftKVService，返回 Raft 节点引用
func buildRaftKVService(
	appCfg *configs.AppConfig,
	st storage.Storage,
	remoteClient common.RemoteClient,
) (services.KVService, *raft.Node, raft.Transport, error) {
	sm := &raft_store.KVStateMachine{St: st}    // 状态机
	logStore := raft_store.NewRaftLogStore(st)  // Raft 日志存储
	hsStore := raft_store.NewHardStateStore(st) // Raft 持久化状态存储
	transport, err := raft_grpc.NewGRPCTransport(appCfg.Membership.Peers)
	if err != nil {
		return nil, nil, nil, errors.Error{Type: errors.InternalError, Info: err.Error()}
	}
	// 创建 Raft 节点
	raftNode := raft.NewNode(appCfg, sm, logStore, hsStore, transport)
	return services.NewRaftKVService(st, raftNode, remoteClient), raftNode, transport, nil
}

// 一致性哈希 + Gossip 模式下构造 CHashKVService，返回 Gossip 节点引用
func buildConsHashGossipKVService(
	appCfg *configs.AppConfig,
	st storage.Storage,
	remoteClient common.RemoteClient,
) (services.KVService, *gossip.Node, gossip.Transport, chash.Transport, error) {
	ring := chash.NewHashRing(appCfg)
	gossipTransport, err := gossip_grpc.NewGRPCTransport(appCfg.Membership.Peers)
	if err != nil {
		return nil, nil, nil, nil, errors.Error{Type: errors.InternalError, Info: err.Error()}
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
	return kvService, gossipNode, gossipTransport, chashTransport, nil
}

// 创建并注册 gRPC 服务器，返回服务器列表和监听地址
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
		return []*grpc.Server{gossipGRPCServer, chashGRPCServer},
			[]string{appCfg.Self.GossipGRPCAddress, appCfg.Self.CHashGRPCAddress}, nil
	default:
		// 其他模式不需要 gRPC 服务
		return nil, nil, nil
	}
}

// 生成一个标准 grpc.Server，供外部统一创建
func NewGRPCServer() *grpc.Server {
	return grpc.NewServer()
}
