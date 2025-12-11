package raft_store

import (
	"context"
	"distributed-kv-store/internal/storage"
)

// Raft 节点的持久化状态
type HardState struct {
	Term        uint64
	VotedFor    string
	CommitIndex uint64
}

// Raft 节点持久化状态存储接口
type HardStateStore interface {
	Save(h HardState) error   // 保存 HardState
	Load() (HardState, error) // 加载 HardState
}

// HardState 存储实现
type hardStateStore struct {
	st storage.Storage
}

// 创建 HardStateStore 实例
func NewHardStateStore(st storage.Storage) *hardStateStore {
	return &hardStateStore{st: st}
}

func (m *hardStateStore) Save(h HardState) error {
	return m.st.SaveRaftHardState(context.TODO(), storage.RaftHardState{
		Term:        h.Term,
		VotedFor:    h.VotedFor,
		CommitIndex: h.CommitIndex,
	})
}

func (m *hardStateStore) Load() (HardState, error) {
	hs, err := m.st.LoadRaftHardState(context.TODO())
	if err != nil {
		return HardState{}, err
	}
	return HardState{
		Term:        hs.Term,
		VotedFor:    hs.VotedFor,
		CommitIndex: hs.CommitIndex,
	}, nil
}
