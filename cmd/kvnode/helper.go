package main

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/raft"
	"distributed-kv-store/internal/raft/raft_grpc"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
)

// 生成一个标准 grpc.Server，供外部统一创建
func NewGRPCServer() *grpc.Server {
	return grpc.NewServer()
}

// 启动 Raft gRPC 服务，监听 Self.GRPCAdress
func startRaftGRPCServer(appCfg *configs.AppConfig, node *raft.Node) error {
	addr := appCfg.Self.RaftGRPCAddress
	if addr == "" {
		return fmt.Errorf("grpc address not configured")
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	srv := raft_grpc.NewRaftGRPCServer(node)
	grpcServer := NewGRPCServer()
	// 注册 RaftService 服务
	raft_grpc.RegisterRaftServiceServer(grpcServer, srv)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("raft grpc server stopped: %v", err)
		}
	}()

	return nil
}
