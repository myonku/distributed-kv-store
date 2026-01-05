package api

import (
	"net/http"

	"distributed-kv-store/internal/services"

	"github.com/gin-gonic/gin"
)

type HTTPHandler struct {
	svc services.KVService
}

func NewHTTPHandler(svc services.KVService) *HTTPHandler {
	return &HTTPHandler{svc: svc}
}

func (h *HTTPHandler) Get(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, GetResponse{Error: "missing key"})
		return
	}

	value, err := h.svc.Get(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, GetResponse{Error: err.Error()})
		return
	}
	if value == "" {
		c.JSON(http.StatusNotFound, GetResponse{Error: "not found"})
		return
	}

	c.JSON(http.StatusOK, GetResponse{Value: value})
}

func (h *HTTPHandler) Put(c *gin.Context) {

	var req PutRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, PutResponse{Error: "invalid json"})
		return
	}

	if req.Key == "" {
		c.JSON(http.StatusBadRequest, PutResponse{Error: "missing key"})
		return
	}

	if req.Value == "" {
		c.JSON(http.StatusBadRequest, PutResponse{Error: "missing value"})
		return
	}

	if err := h.svc.Put(c.Request.Context(), req.Key, req.Value); err != nil {
		c.JSON(http.StatusInternalServerError, PutResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, PutResponse{})
}

func (h *HTTPHandler) Delete(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, DeleteResponse{Error: "missing key"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), key); err != nil {
		c.JSON(http.StatusInternalServerError, DeleteResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, DeleteResponse{})
}
