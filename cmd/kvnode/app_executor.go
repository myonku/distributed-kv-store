package main

import (
	"context"
	"fmt"
	"strings"

	"distributed-kv-store/internal/gossip"
	"distributed-kv-store/internal/raft"
)

// 命令解析及执行器
type AppCommandExecutor struct {
	raftNode   *raft.Node
	gossipNode *gossip.Node
}

// 创建新的命令执行器实例
func NewAppCommandExecutor(raftNode *raft.Node, gossipNode *gossip.Node) *AppCommandExecutor {
	return &AppCommandExecutor{raftNode: raftNode, gossipNode: gossipNode}
}

// Help 返回帮助信息
func (e *AppCommandExecutor) Help() string {
	lines := []string{
		"help|-h                        show this help",
		"exit|-q                        stop process",
		"status|-s                      show basic runtime status (stub)",
		"add <id>                       add client node (stub)",
		"rm <id>                        remove client node (stub)",
	}
	return strings.Join(lines, "\n")
}

// Execute 执行解析后的命令
func (e *AppCommandExecutor) Execute(ctx context.Context, cmd ParsedCommand) error {
	_ = ctx
	if e == nil {
		return nil
	}

	switch cmd.Name {
	case "status", "-s":
		// TODO: 输出更完整的运行状态（如 mode、leader、members、ring epoch 等）
		return nil
	case "add":
		if len(cmd.Args) < 1 {
			return fmt.Errorf("usage: add <id>")
		}
		_ = cmd.Args[0]
		// TODO: 解析更多参数（地址/weight），并调用 gossip 成员变更入口。
		// if e.gossipNode != nil { return e.gossipNode.AddMember(...) }
		return nil
	case "remove", "del", "rm":
		if len(cmd.Args) < 1 {
			return fmt.Errorf("usage: remove <id>")
		}
		_ = cmd.Args[0]
		// TODO: 调用 gossip 成员下线入口。
		return nil
	default:
		return fmt.Errorf("unknown command: %s", cmd.Name)
	}
}
