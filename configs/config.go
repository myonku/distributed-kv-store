package configs

import (
	"github.com/BurntSushi/toml"
)

type Mode string        // 运行模式：Raft 强一致复制，或基于一致性哈希的去中心化分片
type ConfChangeType int // 配置变更类型
type MembershipType string

const (
	MembershipStatic MembershipType = "static"
	MembershipRaft   MembershipType = "raft"
	MembershipGossip MembershipType = "gossip"
)

const (
	ConfChangeAddNode ConfChangeType = iota
	ConfChangeRemoveNode
)

const (
	ModeStandalone     Mode = "standalone"
	ModeRaft           Mode = "raft"
	ModeConsHashGossip Mode = "chash_gossip"
)

// 集群配置变更条目
type ClusterConfigChange struct {
	Type ConfChangeType
	Node ClusterNode
}

// Raft 模式的集群配置
type RaftClusterConfig struct {
	ElectionTimeoutMs   int // 选举超时时间（毫秒）
	HeartbeatIntervalMs int // 心跳间隔时间（毫秒）
}

// 集群成员管理配置
type MembershipConfig struct {
	Type  MembershipType // 集群协议类型
	Peers []ClusterNode  // 静态成员列表
}

// Gossip 协议配置
type GossipConfig struct {
	ProbeIntervalMs  int // 探测间隔时间（毫秒）
	ProbeTimeoutMs   int // 探测超时时间（毫秒）
	GossipIntervalMs int // Gossip 传播间隔时间（毫秒）
	Fanout           int // 每轮 Gossip 传播时选择的目标节点数量
	SuspectTimeoutMs int // 节点被标记为可疑的时间（毫秒）
	DeadTimeoutMs    int // 节点被标记为死亡的时间（毫秒）
}

// 一致性哈希模式的集群配置
type CHashClusterConfig struct {
	VirtualNodes      int // 虚拟节点数量
	ReplicationFactor int // 副本因子
}

// 集群中的一个节点（物理进程上的一个“服务节点”）
type ClusterNode struct {
	ID              string // 既作为物理节点 ID，也作为逻辑节点 ID
	ClientAddress   string // 对外 HTTP（物理节点层面使用）
	InternalAddress string // Raft GRPC 或 CHASH RemoteClient 使用地址
	GossipAddress   string // Gossip 协议使用地址
	Weight          int    // 只在一致性哈希模式使用，表示环节点的权重
}

// 底层存储配置，形式待定
type StorageConfig struct {
	Path string
}

// 日志配置
type LoggerConfig struct {
	Enabled    bool   // 是否启用文件日志
	Dir        string // 日志目录（相对于 settings.toml 所在目录）
	Extension  string // 文件扩展名："log" 或 "txt" 等
	Prefix     string // 文件名前缀（可为空）
	Level      string // 最低日志级别："debug" | "info" | "warn" | "error"
	Stdout     bool   // 是否同时输出到标准输出
	TimeFormat string // 时间格式（Go time layout），为空则使用 RFC3339
}

// 顶层应用配置，初始时由settings.toml加载，运行时动态维护内存实例
type AppConfig struct {
	Mode         Mode                // 当前运行模式
	Self         *ClusterNode        // 本节点配置
	Membership   *MembershipConfig   // 集群成员管理配置
	Raft         *RaftClusterConfig  // Raft 集群配置
	CHash        *CHashClusterConfig // 一致性哈希集群配置
	GossipConfig *GossipConfig       // Gossip 协议配置
	Storage      StorageConfig       // 底层存储配置
	Logger       *LoggerConfig       // 日志配置
}

// 从 settings.toml 读取初始配置，返回 AppConfig 实例
func ReadConfig(path string) (*AppConfig, error) {
	appConfig := &AppConfig{}
	if _, err := toml.DecodeFile(path, appConfig); err != nil {
		return nil, err
	}
	return appConfig, nil
}
