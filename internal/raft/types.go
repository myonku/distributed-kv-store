package raft

type Role int

const (
	Follower Role = iota
	Leader
	Candidate
)

// Raft 集群中的节点信息
type RaftPeer struct {
	ID              string
	ClientAddress   string
	RaftGRPCAddress string
}

// 提交结果
type ApplyResult struct {
	Index uint64
	Term  uint64
	Err   error
}

// 心跳结果
type HeartbeatResult struct {
	Term    uint64
	Success bool
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

// 集群成员状态（用于打印）
type MemberStatus struct {
	Peer     RaftPeer
	IsLeader bool
}

// 集群状态（用于打印）
type ClusterStatus struct {
	Self    Status
	Members []MemberStatus
}
