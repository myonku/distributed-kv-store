package main

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft"
	raft_rpc "distributed-kv-store/internal/raft/rpc"
	"distributed-kv-store/internal/services"
	"distributed-kv-store/internal/store"
	"fmt"
	"net"

	"google.golang.org/grpc"
)

// 根据配置的运行模式构造对应的 KVService 实现。
func buildKVService(appCfg *configs.AppConfig) (services.KVService, error) {
	// 初始化底层存储
	st, err := store.NewStorage(appCfg.Self.Storage)
	if err != nil {
		return nil, err
	}

	switch appCfg.Mode {
	case configs.ModeStandalone:
		return services.NewStandaloneKVService(st), nil

	case configs.ModeRaft:
		// TODO: 组装 Raft Node + RaftKVService
		svc, _, err := buildRaftMode(appCfg)
		if err != nil {
			return nil, fmt.Errorf("build raft mode: %w", err)
		}
		return svc, nil

	case configs.ModeConsHash:
		// TODO: 后续在此处组装 CHash Node + CHashKVService
		return nil, fmt.Errorf("mode %q not implemented yet", appCfg.Mode)

	default:
		return nil, errors.UnSupportedMode
	}
}

// Raft 模式下构造 Storage + StateMachine + Node + gRPC server + KVService
func buildRaftMode(appCfg *configs.AppConfig) (services.KVService, func(), error) {
	st, err := store.NewStorage(appCfg.Self.Storage)
	if err != nil {
		return nil, nil, err
	}
	sm := &store.KVStateMachine{St: st}
	transport, err := raft_rpc.NewGRPCTransport(appCfg.Raft.Nodes)
	if err != nil {
		return nil, nil, err
	}
	node := raft.NewNode(*appCfg, sm, transport)
	node.Start()

	// 启动 Raft gRPC server，监听内部端口（可单独配置）
	lis, err := net.Listen("tcp", appCfg.Self.GRPCAdress) // 使用单独的 raft_address
	if err != nil {
		return nil, nil, err
	}

	grpcServer := grpc.NewServer()
	raft_rpc.RegisterRaftServiceServer(grpcServer, raft_rpc.NewRaftGRPCServer(node))

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			// 生产环境可以加日志
		}
	}()
	// 封装成 KVService，给 HTTP 层使用
	svc := services.NewRaftKVService(st, node)

	// 返回 cleanup 函数，便于 main 里优雅退出
	cleanup := func() {
		node.Stop()
		grpcServer.GracefulStop()
		_ = transport.Close()
		_ = lis.Close()
	}

	return svc, cleanup, nil
}
