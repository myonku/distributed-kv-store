package configs

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Mode string        // 运行模式：Raft 强一致复制，或基于一致性哈希的去中心化分片
type StorageMode string // 底层存储模式：基于内存或持久化
type MembershipType string

const (
	StorageModeMemory StorageMode = "memory"
	StorageModeSQLite StorageMode = "sqlite"
)

const (
	MembershipStatic         MembershipType = "static"
	MembershipRaft           MembershipType = "raft"
	MembershipConsHashGossip MembershipType = "chash_gossip"
)

const (
	ModeStandalone     Mode = "standalone"
	ModeRaft           Mode = "raft"
	ModeConsHashGossip Mode = "chash_gossip"
)

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

// 集群中的一个节点（物理进程上的一个“服务节点”，或是逻辑上的一个“虚拟节点”）
type ClusterNode struct {
	ID                string // 既作为物理节点 ID，也作为逻辑节点 ID
	ClientAddress     string // 对外 HTTP（物理节点层面使用）
	RaftGRPCAddress   string // Raft GRPC 通信地址
	CHashGRPCAddress  string // 一致性哈希节点间通信地址
	GossipGRPCAddress string // Gossip 协议节点间通信地址
	Weight            int    // 只在一致性哈希模式使用，表示环节点的权重
}

// 底层存储配置
type StorageConfig struct {
	Mode       StorageMode `toml:"mode"`        // 存储模式：memory | sqlite（为空时由 NewStorage 推断）
	Path       string      `toml:"path"`        // sqlite 下可为目录或具体 db 文件；相对路径以 settings.toml 所在目录为基准
	SQLiteFile string      `toml:"sqlite_file"` // sqlite 下当 Path 为目录/为空时使用的默认文件名
	BaseDir    string      `toml:"-"`           // 用于解析相对路径的基准目录，不从 toml 加载
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

	baseDir := filepath.Dir(path)
	if baseDir == "." {
		if wd, err := os.Getwd(); err == nil {
			baseDir = wd
		}
	}
	appConfig.Storage.BaseDir = baseDir
	if appConfig.Storage.SQLiteFile == "" {
		appConfig.Storage.SQLiteFile = "data.db"
	}
	return appConfig, nil
}
