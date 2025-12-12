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

	// 此处的配置对象可能在运行时动态更新（内存中），需传递指针
	appCfg, err := configs.ReadConfig(configPath)
	if err != nil {
		log.Fatalf("read config failed: %v", err)
	}

	// TODO: 构造 gRPC server，并注册 Raft Transport 服务

	// 根据运行模式选择 KVService 实现
	_, svc, err := buildKVService(appCfg) // 返回的 st 需要在 main 中清理
	if err != nil {
		log.Fatalf("build kv service failed: %v", err)
	}

	ctx, cancel := signalContext()
	// defer 清理资源
	// TODO: 根据模式选择清理存储、Node、gRPC server 等
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
