package configs

import (
	"github.com/BurntSushi/toml"
)

// 运行模式：Raft 强一致复制，或基于一致性哈希的去中心化分片
type Mode string

type ConfChangeType int

const (
	ConfChangeAddNode ConfChangeType = iota
	ConfChangeRemoveNode
)

const (
	ModeStandalone Mode = "standalone"
	ModeRaft       Mode = "raft"
	ModeConsHash   Mode = "chash"
)

// Raft 集群配置变更
type ClusterConfigChange struct {
	Type ConfChangeType
	Node RaftPeer
}

// 单个节点“自身”的配置（通用）
type NodeConfig struct {
	ID          string // 本节点 ID
	HTTPAddress string // 对外服务地址（HTTP）
	GRPCAdress  string // Raft 节点间通信地址（gRPC）
	Storage     StorageConfig
}

// Raft 模式的集群配置
type RaftClusterConfig struct {
	// 集群中所有 Raft 节点
	Nodes []RaftPeer
	// 选举超时时间（毫秒）
	ElectionTimeoutMs int
	// 心跳间隔时间（毫秒）
	HeartbeatIntervalMs int
	// 维护当前Raft Leader ID
	LeaderID string
}

type RaftPeer struct {
	ID          string
	GRPCAddress string // gRPC 地址
}

// 一致性哈希模式的集群配置
type CHashClusterConfig struct {
	// Hash ring 上的所有节点
	Nodes []CHashNode
	// 虚拟节点数量
	VirtualNodes int
}

type CHashNode struct {
	ID      string
	Address string
	Weight  int
}

type StorageConfig struct {
	Path string
}

// 顶层应用配置，初始时由settings.toml加载，运行时动态维护内存实例
type AppConfig struct {
	// 当前运行模式
	Mode Mode
	// 本节点
	Self NodeConfig
	// 按模式划分的集群配置（只会用到一个）
	Raft  *RaftClusterConfig
	CHash *CHashClusterConfig
}

// 从 settings.toml 读取初始配置，返回 AppConfig 实例
func ReadConfig(path string) (*AppConfig, error) {
	appConfig := &AppConfig{}
	if _, err := toml.DecodeFile(path, appConfig); err != nil {
		return nil, err
	}
	return appConfig, nil
}
