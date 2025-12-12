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
)

// 根据配置的运行模式构造对应的 KVService 实现。
func buildKVService(appCfg *configs.AppConfig) (storage.Storage, services.KVService, error) {
	st, err := storage.NewStorage(appCfg.Self.Storage)
	if err != nil {
		return nil, nil, err
	}

	switch appCfg.Mode {
	case configs.ModeStandalone:
		return st, services.NewStandaloneKVService(st), nil

	case configs.ModeRaft:
		svc, err := buildRaftMode(appCfg)
		if err != nil {
			return st, nil, fmt.Errorf("build raft mode: %w", err)
		}
		return st, svc, nil

	case configs.ModeConsHash:
		// TODO: 后续在此处组装 CHash Node + CHashKVService
		return st, nil, fmt.Errorf("mode %q not implemented yet", appCfg.Mode)

	default:
		st.Close()
		return nil, nil, errors.UnSupportedMode
	}
}

// Raft 模式下构造 Storage + StateMachine + Node + gRPC server + KVService
func buildRaftMode(appCfg *configs.AppConfig) (services.KVService, error) {
	st, err := storage.NewStorage(appCfg.Self.Storage)
	if err != nil {
		return nil, err
	}
	sm := &raft_store.KVStateMachine{St: st}
	transport, err := raft_grpc.NewGRPCTransport(appCfg.Raft.Nodes)
	if err != nil {
		return nil, err
	}
	hsStore, logStore := raft_store.NewHardStateStore(st), raft_store.NewRaftLogStore(st)
	node := raft.NewNode(*appCfg, sm, logStore, hsStore, transport)
	// 加载持久化状态
	if err := node.LoadState(); err != nil {
		return nil, errors.ErrCannotLoadState
	}
	node.Start()
	return services.NewRaftKVService(st, node), nil
}
