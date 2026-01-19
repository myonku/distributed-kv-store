package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"distributed-kv-store/configs"
	"distributed-kv-store/internal/api"
	"distributed-kv-store/internal/util"

	"google.golang.org/grpc"
)

func main() {
	// 启动参数（骨架）：后续可扩展更多运行时/启动时控制开关。
	configPath, consoleEnabled, initialCmds := parseStartupFlags(os.Args[1:])

	appCfg, err := configs.ReadConfig(configPath)
	if err != nil {
		log.Fatalf("read config failed: %v", err)
	}

	// 初始化全局日志（按天写文件到 <settings.toml>/logs/ ）
	initGlobalLogger(configPath, appCfg)

	// 根据运行模式选择 KVService 实现
	st, svc, raftNode, gossipNode, ring, cleanup, err := buildKVService(appCfg)
	if err != nil {
		log.Fatalf("build kv service failed: %v", err)
	}
	defer cleanup()

	// 创建 gRPC 服务器
	grpcServers, grpcAddrs, err := buildGRPCServer(appCfg, st, raftNode, gossipNode, ring)
	if err != nil {
		log.Fatalf("build grpc server failed: %v", err)
	}

	// 启动 HTTP API 服务器与后台组件统一受 signal ctx 控制
	ctx, cancel := signalContext()
	defer cancel()
	defer st.Close()
	defer svc.Dispose()

	// 启动 gRPC 服务器
	grpcStop, err := startGRPCServers(ctx, grpcServers, grpcAddrs)
	if err != nil {
		log.Fatalf("start grpc server failed: %v", err)
	}
	defer grpcStop()

	// 启动 service 内部资源（如 Raft 节点、Gossip 节点等）
	svc.RunService()

	// 运行时命令入口（骨架）：监听 stdin，解析并分发命令
	if consoleEnabled {
		exec := NewAppCommandExecutor(raftNode, gossipNode)
		startCommandConsole(ctx, cancel, exec, initialCmds)
	}

	// 启动 HTTP API 服务器
	if err := api.StartHTTPServer(ctx, appCfg.Self.ClientAddress, svc); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

// 根据运行模式构造 KVService 实例并启动，返回清理函数
func startGRPCServers(ctx context.Context, servers []*grpc.Server, addrs []string) (func(), error) {
	listeners := make([]net.Listener, 0, len(servers))
	for i, srv := range servers {
		addr := addrs[i]
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			for _, l := range listeners {
				_ = l.Close()
			}
			return func() {}, err
		}
		listeners = append(listeners, lis)
		go func(srv *grpc.Server, lis net.Listener) {
			if err := srv.Serve(lis); err != nil {
				log.Printf("grpc server stopped: %v", err)
			}
		}(srv, lis)
		log.Printf("gRPC server started at %s", addr)
	}

	stop := func() {
		// 先关闭 listener，阻止新连接
		for _, lis := range listeners {
			_ = lis.Close()
		}
		// 尝试优雅停止，超时后强制 Stop
		for _, srv := range servers {
			done := make(chan struct{})
			go func(s *grpc.Server) {
				s.GracefulStop()
				close(done)
			}(srv)
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				srv.Stop()
			}
		}
	}

	go func() {
		<-ctx.Done()
		stop()
	}()

	return stop, nil
}

// 初始化全局日志
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

// signalContext 返回一个在接收到中断/终止信号时关闭的 Context
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
