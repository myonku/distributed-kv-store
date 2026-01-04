package raft

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/raft/raft_store"
	"sync"
	"time"
)

// Raft 节点
type Node struct {
	mu sync.RWMutex

	id          string
	role        Role
	peers       map[string]RaftPeer // 运行时成员视图
	term        uint64              // 当前任期
	votedFor    string              // 当前任期内投票给的节点 ID
	commitIndex uint64              // 已提交的最高日志索引
	lastApplied uint64              // 最近一次应用到状态机的日志索引
	voteCount   int                 // 当前任期内已获得的选票数（包含自己）
	leaderID    string              // 当前任期内认为的 Leader 节点 ID

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
	electionResetAt  time.Time     // 最近一次收到 leader 心跳/有效 RPC 的时间（用于抑制误触发选举）

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

	peersMap := make(map[string]RaftPeer, len(cfg.Membership.Peers))
	for _, p := range cfg.Membership.Peers {
		peersMap[p.ID] = RaftPeer{
			ID:              p.ID,
			ClientAddress:   p.ClientAddress,
			RaftGRPCAddress: p.RaftGRPCAddress,
		}
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
		electionResetAt:  time.Now(),
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
	if n == nil || n.cancel == nil {
		return
	}
	n.cancel()
}
