package raft

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/raft/raft_store"
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
	peers map[string]configs.RaftPeer // 运行时成员视图

	term        uint64
	votedFor    string
	commitIndex uint64
	lastApplied uint64
	voteCount   int // 当前任期内已获得的选票数（包含自己）
	leaderID    string

	// leader 才使用
	nextIndex  map[string]uint64 // 下一个要发送给该 follower 的日志条目索引
	matchIndex map[string]uint64 // 已知该 follower 已复制的最高日志条目索引

	logStore       raft_store.RaftLogStore   // 日志存储
	hardStateStore raft_store.HardStateStore // term / votedFor / commitIndex 等持久化状态

	sm        raft_store.StateMachine // 底层状态机（KV 状态机）
	transport Transport               // 网络层
	applyCh   chan ApplyResult

	electionTimeout  time.Duration // 选举超时
	heartbeatTimeout time.Duration // 心跳间隔

	ctx    context.Context
	cancel context.CancelFunc
}

// 创建一个新的 Raft 节点实例
func NewNode(
	cfg *configs.AppConfig,
	sm raft_store.StateMachine,
	logStore raft_store.RaftLogStore,
	hardStateStore raft_store.HardStateStore,
	transport Transport,
) *Node {
	ctx, cancel := context.WithCancel(context.Background())

	peersMap := make(map[string]configs.RaftPeer, len(cfg.Raft.Nodes))
	for _, p := range cfg.Raft.Nodes {
		peersMap[p.ID] = p
	}

	n := &Node{
		id:               cfg.Self.ID,
		role:             Follower,
		peers:            peersMap,
		nextIndex:        make(map[string]uint64),
		matchIndex:       make(map[string]uint64),
		logStore:         logStore,
		hardStateStore:   hardStateStore,
		sm:               sm,
		transport:        transport,
		electionTimeout:  time.Duration(cfg.Raft.ElectionTimeoutMs),
		heartbeatTimeout: time.Duration(cfg.Raft.HeartbeatIntervalMs),
		ctx:              ctx,
		cancel:           cancel,
	}
	n.applyCh = make(chan ApplyResult, 100)
	return n
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
