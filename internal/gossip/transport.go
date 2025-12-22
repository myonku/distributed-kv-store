package gossip

import "context"

type Transport interface {
	// 发送 Ping 消息
	Ping(ctx context.Context, to string, req *PingRequest) (*PingResponse, error)
	// 发送 PushPull 消息
	GossipPushPull(ctx context.Context, to string, req *PushPullRequest) (*PushPullResponse, error)
}

// Ping 请求与响应

type PingRequest struct {
	FromID          string
	FromIncarnation uint64
}

type PingResponse struct {
	OK bool
}

// PushPull 请求与响应

type PushPullRequest struct {
	FromID      string
	Digests     []Digest
	FullMembers []Member
}

type PushPullResponse struct {
	Delta []Member // 需要更新的成员信息
}
