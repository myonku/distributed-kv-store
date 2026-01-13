package main

import (
	"context"
)

// CommandExecutor 定义命令执行接口
type CommandExecutor interface {
	Execute(ctx context.Context, cmd ParsedCommand) error
	Help() string
}

// 表示解析后的命令
type ParsedCommand struct {
	Name string
	Args []string
	Raw  string
}
