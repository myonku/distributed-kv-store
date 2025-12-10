package raft

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/store"
	"sync"
	"time"
)

type Role int

const (
	Follower Role = iota
	Leader
	Candidate
)

type ApplyResult struct {
	Index uint64
	Term  uint64
	Err   error
}

// 供上层查询的节点状态快照
type Status struct {
	ID            string
	Role          Role
	Term          uint64
	CommitIndex   uint64
	LastApplied   uint64
	CurrentLeader string
}

// Raft 节点
type Node struct {
	mu sync.RWMutex

	id    string
	role  Role
	cfg   *configs.AppConfig // 配置引用
	peers []configs.RaftPeer

	term        uint64
	votedFor    string
	log         []LogEntry
	commitIndex uint64
	lastApplied uint64
	voteCount   int // 当前任期内已获得的选票数（包含自己）

	// leader 才使用
	nextIndex  map[string]uint64 // 下一个要发送给该 follower 的日志条目索引
	matchIndex map[string]uint64 // 已知该 follower 已复制的最高日志条目索引

	sm        store.StateMachine // 底层状态机
	transport Transport          // 网络层
	applyCh   chan ApplyResult

	// 选举、心跳相关
	electionTimeout  time.Duration
	heartbeatTimeout time.Duration

	// 关闭控制
	ctx    context.Context
	cancel context.CancelFunc
}

// 创建一个新的 Raft 节点实例
func NewNode(cfg configs.AppConfig, sm store.StateMachine, transport Transport) *Node {
	ctx, cancel := context.WithCancel(context.Background())

	node := &Node{
		id:         cfg.Self.ID,
		role:       Follower,
		cfg:        &cfg,
		peers:      cfg.Raft.Nodes,
		log:        make([]LogEntry, 0),
		nextIndex:  make(map[string]uint64),
		matchIndex: make(map[string]uint64),
		sm:         sm,
	}
	node.transport = transport
	node.applyCh = make(chan ApplyResult, 100)
	node.electionTimeout = time.Duration(cfg.Raft.ElectionTimeoutMs) * time.Millisecond
	node.heartbeatTimeout = time.Duration(cfg.Raft.HeartbeatIntervalMs) * time.Millisecond
	node.ctx = ctx
	node.cancel = cancel
	return node
}

// 启动内部 goroutine（选举、日志复制）
func (n *Node) Start() {
	// 起选举循环、心跳循环等
	go n.runElectionLoop()
	go n.runHeartbeatLoop()
	go n.runApplyLoop()
}

// 停止节点
func (n *Node) Stop() {
	n.cancel()
}
