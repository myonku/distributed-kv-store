package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"distributed-kv-store/configs"
	"distributed-kv-store/internal/api"
	"distributed-kv-store/internal/services"
	"distributed-kv-store/internal/store"
)

// kvnode 启动入口

func main() {
	// 配置文件路径，默认使用工作目录下的 settings.toml
	configPath := "settings.toml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	appCfg, err := configs.ReadConfig(configPath)
	if err != nil {
		log.Fatalf("read config failed: %v", err)
	}

	// 根据运行模式选择 KVService 实现
	svc, err := buildKVService(appCfg)
	if err != nil {
		log.Fatalf("build kv service failed: %v", err)
	}

	ctx, cancel := signalContext()
	defer cancel()

	if err := api.StartHTTPServer(ctx, appCfg.Self.Address, svc); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

// signalContext 返回一个在接收到中断/终止信号时关闭的 Context。
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		cancel()
	}()

	return ctx, cancel
}

// 根据配置的运行模式构造对应的 KVService 实现。
func buildKVService(appCfg *configs.AppConfig) (api.KVService, error) {
	// 初始化底层存储（目前为纯内存实现，忽略路径）
	st, err := store.NewStorage(appCfg.Self.Storage)
	if err != nil {
		return nil, err
	}

	switch appCfg.Mode {
	case configs.ModeStandalone:
		kvStore := store.NewStandaloneKVStore(st)
		return services.NewStandaloneKVService(kvStore), nil

	case configs.ModeRaft:
		// TODO: 后续在此处组装 Raft Node + RaftKVService
		return nil, fmt.Errorf("mode %q not implemented yet", appCfg.Mode)

	case configs.ModeConsHash:
		// TODO: 后续在此处组装 CHash Node + CHashKVService
		return nil, fmt.Errorf("mode %q not implemented yet", appCfg.Mode)

	default:
		return nil, fmt.Errorf("unknown mode %q", appCfg.Mode)
	}
}
