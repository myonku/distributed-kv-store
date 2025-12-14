package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"distributed-kv-store/configs"
	"distributed-kv-store/internal/api"
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
	_, svc, raftNode, err := buildKVService(appCfg) // node 仅在非单机模式下使用

	if err != nil {
		log.Fatalf("build kv service failed: %v", err)
	}

	// 非单机模式下，还需启动 Raft 节点相关的 gRPC 服务
	if appCfg.Mode == configs.ModeRaft || appCfg.Mode == configs.ModeConsHash {
		if err := startRaftGRPCServer(appCfg, raftNode); err != nil {
			return
		}
	}

	ctx, cancel := signalContext()
	defer cancel()

	if err := api.StartHTTPServer(ctx, appCfg.Self.HTTPAddress, svc); err != nil {
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
