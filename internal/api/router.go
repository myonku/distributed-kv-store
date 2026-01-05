package api

import (
	"distributed-kv-store/internal/services"

	"github.com/gin-gonic/gin"
)

func NewRouter(svc services.KVService) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	h := NewHTTPHandler(svc)
	r.GET("/kv/:key", h.Get)
	r.PUT("/kv", h.Put)
	r.DELETE("/kv/:key", h.Delete)

	return r
}
