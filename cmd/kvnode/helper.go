package main

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/raft/raft_grpc"
	"distributed-kv-store/internal/raft/raft_store"
	"distributed-kv-store/internal/services"
	"distributed-kv-store/internal/storage"
	"fmt"
	"log"
	"net"
)

// 根据配置的运行模式构造对应的 KVService 实现，返回全部内部资源以便调用方管理生命周期
func buildKVService(appCfg *configs.AppConfig) (*storage.Storage, services.KVService, *raft.Node, error) {
	switch appCfg.Mode {
	case configs.ModeStandalone:
		st, err := storage.NewStorage(appCfg.Self.Storage)
		if err != nil {
			return nil, nil, nil, err
		}
		return &st, services.NewStandaloneKVService(st), nil, nil

	case configs.ModeRaft:
		st, svc, node, err := buildRaftMode(appCfg)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("build raft mode: %w", err)
		}
		return st, svc, node, nil

	case configs.ModeConsHash:
		// TODO: 后续在此处组装 CHash Node + CHashKVService
		st, err := storage.NewStorage(appCfg.Self.Storage)
		if err != nil {
			return nil, nil, nil, err
		}
		return &st, nil, nil, fmt.Errorf("mode %q not implemented yet", appCfg.Mode)

	default:
		return nil, nil, nil, errors.ErrUnSupportedMode
	}
}

// Raft 模式下构造 Storage + StateMachine + Node + gRPC server + KVService
func buildRaftMode(appCfg *configs.AppConfig) (*storage.Storage, services.KVService, *raft.Node, error) {
	st, err := storage.NewStorage(appCfg.Self.Storage)
	if err != nil {
		return nil, nil, nil, err
	}
	sm := &raft_store.KVStateMachine{St: st}
	transport, err := raft_grpc.NewGRPCTransport(appCfg.Raft.Nodes)
	if err != nil {
		st.Close()
		return nil, nil, nil, err
	}
	hsStore, logStore := raft_store.NewHardStateStore(st), raft_store.NewRaftLogStore(st)
	node := raft.NewNode(appCfg, sm, logStore, hsStore, transport)

	if err := node.LoadState(); err != nil {
		st.Close()
		return nil, nil, nil, fmt.Errorf("load hard state: %w", err)
	}
	node.Start()

	svc := services.NewRaftKVService(st, node)
	return &st, svc, node, nil
}

// 启动 Raft gRPC 服务，监听 Self.GRPCAdress
func startRaftGRPCServer(appCfg *configs.AppConfig, node *raft.Node) error {
	addr := appCfg.Self.GRPCAdress
	if addr == "" {
		return fmt.Errorf("grpc address not configured")
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	srv := raft_grpc.NewRaftGRPCServer(node)
	grpcServer := raft_grpc.NewGRPCServerWrapper()
	// 注册 RaftService 服务
	raft_grpc.RegisterRaftServiceServer(grpcServer, srv)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("raft grpc server stopped: %v", err)
		}
	}()

	return nil
}
