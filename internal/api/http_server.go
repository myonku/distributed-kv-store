package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"distributed-kv-store/internal/services"
)

// Http Server 管理

// StartHTTPServer 启动对外提供 KV 服务的 HTTP Server
func StartHTTPServer(ctx context.Context, addr string, svc services.KVService) error {
	router := NewRouter(svc)

	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	// 优雅关闭
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("http server shutdown error: %v", err)
		}
	}()

	log.Printf("HTTP server listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
