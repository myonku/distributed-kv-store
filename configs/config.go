package configs

import (
	"github.com/BurntSushi/toml"
)

type Mode string            // 运行模式：Raft 强一致复制，或基于一致性哈希的去中心化分片
type NodeRole string        // 节点角色：控制面节点或数据面节点
type ConfChangeType int     // 配置变更类型
type ConfChangeScope string // 配置变更作用域

const (
	ConfChangeAddNode ConfChangeType = iota
	ConfChangeRemoveNode
)

const (
	NodeRoleUnknown      NodeRole = ""
	NodeRoleControlPlane NodeRole = "control" // 参与控制面 Raft
	NodeRoleData         NodeRole = "data"    // 参与数据存储/分片
)

const (
	ModeStandalone Mode = "standalone"
	ModeRaft       Mode = "raft"
	ModeConsHash   Mode = "chash"
)

const (
	ConfChangeScopeCluster ConfChangeScope = "control" // 控制面节点，由 Raft 管理
	ConfChangeScopeNode    ConfChangeScope = "data"    // CHASH 数据面节点
)

// 集群配置变更条目
type ClusterConfigChange struct {
	Type  ConfChangeType
	Node  ClusterNode
	Scope ConfChangeScope
}

// Raft 模式的集群配置
type RaftClusterConfig struct {
	Nodes               []ClusterNode // 集群中所有 Raft 节点
	ElectionTimeoutMs   int           // 选举超时时间（毫秒）
	HeartbeatIntervalMs int           // 心跳间隔时间（毫秒）
	LeaderID            string        // 维护当前Raft Leader ID
}

// 一致性哈希模式的集群配置
type CHashClusterConfig struct {
	Nodes        []ClusterNode // Hash ring 上的所有节点
	VirtualNodes int           // 虚拟节点数量
}

// 集群中的一个逻辑节点（物理进程上的一个“服务节点”），不强行区分是 Raft peer 还是 CHash node
type ClusterNode struct {
	ID          string   // 既作为物理节点 ID，也作为逻辑节点 ID
	HTTPAddress string   // 对外 HTTP（物理节点层面使用）
	GRPCAddress string   // 内部逻辑节点间 Raft / RPC
	Role        NodeRole // 逻辑节点角色，表明逻辑节点参与的集群类型
	Weight      int      // 只在一致性哈希模式使用，表示该节点的权重
}

// 底层存储配置
type StorageConfig struct {
	Path string
}

// 顶层应用配置，初始时由settings.toml加载，运行时动态维护内存实例
type AppConfig struct {
	Mode    Mode                // 当前运行模式
	Self    *ClusterNode        // 本节点配置
	Raft    *RaftClusterConfig  // Raft 集群配置，在主从模式用作复制，在CHASH模式用作控制面
	CHash   *CHashClusterConfig // 一致性哈希集群配置
	Storage StorageConfig       // 底层存储配置
}

// 从 settings.toml 读取初始配置，返回 AppConfig 实例
func ReadConfig(path string) (*AppConfig, error) {
	appConfig := &AppConfig{}
	if _, err := toml.DecodeFile(path, appConfig); err != nil {
		return nil, err
	}
	return appConfig, nil
}
