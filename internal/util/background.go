package util

import (
	"context"
	"distributed-kv-store/internal/storage"
)

// BackgroundTaskManager 管理多个后台任务
type BackgroundTaskManager struct {
	tasks map[string]BackgroundTask
	st    storage.Storage

	ctx    context.Context
	cancel context.CancelFunc
}

// NewBackgroundTaskManager 创建一个新的后台任务管理器
func NewBackgroundTaskManager(st storage.Storage) *BackgroundTaskManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &BackgroundTaskManager{
		tasks:  make(map[string]BackgroundTask),
		st:     st,
		ctx:    ctx,
		cancel: cancel,
	}
}

// RegisterTask 注册一个后台任务
func (m *BackgroundTaskManager) RegisterTask(name string, task BackgroundTask) {
	m.tasks[name] = task
}

// 启动某个后台任务
func (m *BackgroundTaskManager) StartTask(name string) error {
	task, exists := m.tasks[name]
	if !exists {
		return nil
	}
	return task.Start()
}

// 停止某个后台任务
func (m *BackgroundTaskManager) StopTask(name string) error {
	task, exists := m.tasks[name]
	if !exists {
		return nil
	}
	return task.Stop()
}

// StartAll 启动所有注册的后台任务
func (m *BackgroundTaskManager) StartAll() error {
	for _, task := range m.tasks {
		if err := task.Start(); err != nil {
			return err
		}
	}
	return nil
}

// StopAll 停止所有注册的后台任务
func (m *BackgroundTaskManager) StopAll() error {
	for _, task := range m.tasks {
		if err := task.Stop(); err != nil {
			return err
		}
	}
	return nil
}

// Dispose 释放管理器资源，停止所有任务
func (m *BackgroundTaskManager) Dispose() {
	defer m.cancel()
	_ = m.StopAll()
}

// 执行匿名后台任务，随管理器生命周期启动和停止
func (m *BackgroundTaskManager) RunAnonymousTask(taskFunc func(ctx context.Context, args ...any), args ...any) {
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel
	go func() {
		taskFunc(ctx, args...)
	}()
}
