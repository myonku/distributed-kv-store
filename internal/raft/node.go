package raft

import (
	"distributed-kv-store/configs"
	"sync"
)

type Role int

const (
	Follower Role = iota
	Leader
	Candidate
)

// 表示 Raft 集群中的一个节点
type Node struct {
	mu sync.Mutex

	id    string
	role  Role
	cfg   *configs.NodeConfig
	peers *[]configs.RaftPeer

	term        uint64
	commitIndex uint64
	lastApplied uint64
}

func NewNode(cfg *configs.NodeConfig, peers *[]configs.RaftPeer) *Node {
	return &Node{
		id:          cfg.ID,
		role:        Follower, // 初始为 Follower
		term:        0,
		commitIndex: 0,
		lastApplied: 0,
		cfg:         cfg,
		peers:       peers,
	}
}
