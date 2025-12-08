package api

import (
	"context"
	"distributed-kv-store/internal/services"
	"io"
	"log"
	"net/http"
	"time"
)

// Http Server 管理

// StartHTTPServer 启动对外提供 KV 服务的 HTTP Server。
func StartHTTPServer(ctx context.Context, addr string, svc services.KVService) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/kv", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("missing key"))
			return
		}

		switch r.Method {
		case http.MethodPut: // PUT路由约定为写入操作
			valueBytes, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("read body error"))
				return
			}
			defer r.Body.Close()

			if err := svc.Put(r.Context(), key, string(valueBytes)); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}
			w.WriteHeader(http.StatusNoContent)

		case http.MethodGet: // GET路由约定为读取操作
			value, err := svc.Get(r.Context(), key)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}
			if value == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(value))

		case http.MethodDelete: // DELETE路由约定为删除操作
			if err := svc.Delete(r.Context(), key); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(err.Error()))
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
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
