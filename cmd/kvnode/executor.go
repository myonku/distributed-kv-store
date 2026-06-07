package main

import (
	"context"
	"fmt"
	"strings"

	"distributed-kv-store/configs"
	"distributed-kv-store/internal/common"
	"distributed-kv-store/internal/errors"
	"distributed-kv-store/internal/gossip"
	"distributed-kv-store/internal/raft"
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
	cmds := []string{
		"help|-h",
		"exit|-q",
		"status|-s",
		"meet|-m <id> <client_addr> <internal_addr1> <internal_addr2>",
		"forget|-f|-r <id>",
	}
	intros := []string{
		"显示帮助信息",
		"退出当前进程",
		"显示当前节点的状态信息",
		"添加服务器节点到集群。internal_addr1可以是Raft或Gossip逻辑节点的内部通信地址，internal_addr2只能作为ConsHash逻辑节点的内部通信地址",
		"从集群中移除服务器节点，可以从 status 命令中查看节点 ID",
	}
	var builder strings.Builder
	for i, cmd := range cmds {
		fmt.Fprintf(&builder, "  %-40s : %s\n", cmd, intros[i])
	}
	return builder.String()
}

// Execute 执行解析后的命令
func (e *AppCommandExecutor) Execute(ctx context.Context, cmd ParsedCommand) error {

	if e == nil {
		return nil
	}

	switch cmd.Name {
	case "status", "-s":
		if e.raftNode != nil {
			info := e.raftNode.FormatClusterStatus()
			fmt.Println("Cluster Status:\n" + info)
		} else if e.gossipNode != nil {
			info := e.gossipNode.FormatClusterStatus()
			fmt.Println("Cluster Status:\n" + info)
		} else {
			fmt.Println("Running in standalone mode, no cluster status available.")
		}
		return nil
	case "meet", "-m":
		if e.raftNode == nil && e.gossipNode == nil {
			return errors.Error{
				Type: errors.ConditionError,
				Info: "not to support adding node in current mode",
			}
		}
		if len(cmd.Args) < 3 {
			return errors.Error{
				Type: errors.InvalidArgument,
				Info: "usage: meet <id> <client_addr> <internal_addr1> <internal_addr2>",
			}
		}
		id := cmd.Args[0]
		clientAddr := cmd.Args[1]
		internalAddr1 := cmd.Args[2]
		internalAddr2 := ""
		if len(cmd.Args) >= 4 {
			internalAddr2 = cmd.Args[3]
		}
		// 构造新节点信息并提交配置变更
		node := configs.ClusterNode{
			ID:                id,
			ClientAddress:     clientAddr,
			RaftGRPCAddress:   internalAddr1,
			GossipGRPCAddress: internalAddr1,
			CHashGRPCAddress:  internalAddr2,
		}
		cc := common.ClusterConfigChange{Type: common.ConfChangeAddNode, Node: node}
		// 提交配置变更
		if e.raftNode != nil {
			_, err := e.raftNode.ProposeConfChange(ctx, cc)
			return err
		} else if e.gossipNode != nil {
			return e.gossipNode.ApplyConfChange(cc)
		}
		return nil
	case "-f", "-r", "forget":
		if len(cmd.Args) < 1 {
			return errors.Error{
				Type: errors.InvalidArgument,
				Info: "usage: forget <id>",
			}
		}
		if e.raftNode == nil && e.gossipNode == nil {
			return errors.Error{
				Type: errors.ConditionError,
				Info: "not to support removing node in current mode",
			}
		}
		id := cmd.Args[0]
		// 构造配置变更并提交
		cc := common.ClusterConfigChange{
			Type: common.ConfChangeRemoveNode,
			Node: configs.ClusterNode{ID: id},
		}
		if e.raftNode != nil {
			_, err := e.raftNode.ProposeConfChange(ctx, cc)
			return err
		} else if e.gossipNode != nil {
			return e.gossipNode.ApplyConfChange(cc)
		}
		return nil
	default:
		return errors.Error{
			Type: errors.InvalidArgument,
			Info: fmt.Sprintf("unknown command: %s", cmd.Name),
		}
	}
}
