package configs

import (
	"github.com/BurntSushi/toml"
)

// 运行模式：Raft 强一致复制，或基于一致性哈希的去中心化分片
type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModeRaft       Mode = "raft"
	ModeConsHash   Mode = "chash"
)

// 单个节点“自身”的配置（通用）
type NodeConfig struct {
	ID      string // 本节点 ID
	Address string // 对外服务地址（HTTP/RPC）
	Storage StorageConfig
}

// Raft 模式的集群配置
type RaftClusterConfig struct {
	// 集群中所有 Raft 节点
	Nodes []RaftPeer
	// 选举超时时间（毫秒）
	ElectionTimeoutMs int
	// 心跳间隔时间（毫秒）
	HeartbeatIntervalMs int
}

type RaftPeer struct {
	ID      string
	Address string
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

// 顶层应用配置
type AppConfig struct {
	// 当前运行模式
	Mode Mode
	// 本节点
	Self NodeConfig
	// 按模式划分的集群配置（只会用到一个）
	Raft  *RaftClusterConfig
	CHash *CHashClusterConfig
}

// 从 settings.toml 读取配置
func ReadConfig(path string) (*AppConfig, error) {
	appConfig := &AppConfig{}
	if _, err := toml.DecodeFile(path, appConfig); err != nil {
		return nil, err
	}
	return appConfig, nil
}
