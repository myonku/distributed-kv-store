package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"distributed-kv-store/configs"
	"distributed-kv-store/internal/api"
	"distributed-kv-store/internal/util"
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

	// 初始化全局日志（按天写文件到 <settings.toml>/logs/ ）
	initGlobalLogger(configPath, appCfg)

	// 根据运行模式选择 KVService 实现
	_, svc, raftNode, err := buildKVService(appCfg) // node 仅在非单机模式下使用

	if err != nil {
		log.Fatalf("build kv service failed: %v", err)
	}

	// 非单机模式下，还需启动 Raft 节点相关的 gRPC 服务
	if appCfg.Mode == configs.ModeRaft {
		if err := startRaftGRPCServer(appCfg, raftNode); err != nil {
			return
		}
	}

	ctx, cancel := signalContext()
	defer cancel()

	if err := api.StartHTTPServer(ctx, appCfg.Self.ClientAddress, svc); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

func initGlobalLogger(configPath string, appCfg *configs.AppConfig) {
	baseDir := filepath.Dir(configPath)
	if baseDir == "." {
		if wd, err := os.Getwd(); err == nil {
			baseDir = wd
		}
	}

	// 默认配置
	cfg := &configs.LoggerConfig{
		Enabled:   true,
		Dir:       "logs",
		Extension: "log",
		Prefix:    "kvnode",
		Level:     "info",
		Stdout:    true,
	}
	if appCfg != nil && appCfg.Logger != nil {
		cfg = appCfg.Logger
	}
	if !cfg.Enabled {
		util.SetGlobalLogger(nil)
		return
	}

	l, err := util.NewDailyFileLogger(util.DailyFileLoggerOptions{
		BaseDir:    baseDir,
		Dir:        cfg.Dir,
		Extension:  cfg.Extension,
		Prefix:     cfg.Prefix,
		MinLevel:   util.ParseLogLevel(cfg.Level),
		Stdout:     cfg.Stdout,
		TimeFormat: cfg.TimeFormat,
	})
	if err != nil {
		log.Printf("init file logger failed: %v", err)
		return
	}
	util.SetGlobalLogger(l)
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
