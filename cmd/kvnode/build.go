package main

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/services"
	"distributed-kv-store/internal/storage"
	"fmt"
)

// 根据配置的运行模式构造对应的 KVService 实现，返回全部内部资源以便调用方管理生命周期
func buildKVService(appCfg *configs.AppConfig) (*storage.Storage, services.KVService, *raft.Node, error) {
	switch appCfg.Mode {
	case configs.ModeStandalone:
		st, err := storage.NewStorage(appCfg.Storage)
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

	case configs.ModeConsHashGossip:
		// TODO: 后续在此处组装 CHash Node + CHashKVService
		st, err := storage.NewStorage(appCfg.Storage)
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
	return nil, nil, nil, fmt.Errorf("not implemented yet")
}
