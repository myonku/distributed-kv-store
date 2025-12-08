package raft

import (
	"context"
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/store"
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

	kv store.KVStore // 本机状态机/存储
}

func NewNode(cfg *configs.NodeConfig, peers *[]configs.RaftPeer, kv store.KVStore) *Node {
	return &Node{
		id:          cfg.ID,
		role:        Follower, // 初始为 Follower
		term:        0,
		commitIndex: 0,
		lastApplied: 0,
		cfg:         cfg,
		peers:       peers,
		kv:          kv,
	}
}

func (n *Node) Put(ctx context.Context, key, value string) error {
	return n.kv.Put(ctx, key, value)
}

func (n *Node) Get(ctx context.Context, key string) (string, error) {
	return n.kv.Get(ctx, key)
}

func (n *Node) Delete(ctx context.Context, key string) error {
	return n.kv.Delete(ctx, key)
}
