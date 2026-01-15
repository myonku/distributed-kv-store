package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"distributed-kv-store/configs"
	"distributed-kv-store/internal/common"

	_ "modernc.org/sqlite"
)

// 本地存储实现，基于 SQLite
type localStorage struct {
	mu sync.RWMutex
	db *sql.DB

	lastKVIndex   uint64
	lastRaftIndex uint64
}

// 返回新的本地存储实例的引用
func newLocalStorage(cfg configs.StorageConfig) (Storage, error) {
	dbPath, err := resolveSQLiteDBPath(cfg)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	// sqlite 更适合单连接池（避免多连接下锁竞争/事务隔离差异）
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	st := &localStorage{db: db}
	if err := st.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := st.loadLastIndexes(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

// 初始化数据库 schema
func (s *localStorage) init(ctx context.Context) error {
	// PRAGMA
	if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON;"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout = 5000;"); err != nil {
		return err
	}
	// WAL 会返回一行结果，但 ExecContext 也可正常触发。
	if _, err := s.db.ExecContext(ctx, "PRAGMA journal_mode = WAL;"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA synchronous = NORMAL;"); err != nil {
		return err
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS kv_state (
			k TEXT PRIMARY KEY,
			v TEXT NOT NULL,
			hash INTEGER NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS kv_log (
			idx INTEGER PRIMARY KEY AUTOINCREMENT,
			op TEXT NOT NULL,
			k TEXT NOT NULL,
			v TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_kv_log_idx ON kv_log(idx);`,
		`CREATE INDEX IF NOT EXISTS idx_kv_state_hash ON kv_state(hash);`,

		`CREATE TABLE IF NOT EXISTS raft_log (
			idx INTEGER PRIMARY KEY,
			term INTEGER NOT NULL,
			entry_type INTEGER NOT NULL,
			cmd_op TEXT NOT NULL,
			cmd_k TEXT NOT NULL,
			cmd_v TEXT NOT NULL,
			conf_json TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_raft_log_idx ON raft_log(idx);`,

		`CREATE TABLE IF NOT EXISTS raft_hardstate (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			term INTEGER NOT NULL,
			voted_for TEXT NOT NULL,
			commit_index INTEGER NOT NULL
		);`,

		`CREATE TABLE IF NOT EXISTS move_range_records (
			move_id INTEGER PRIMARY KEY,
			created_at TEXT NOT NULL
		);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

// 加载当前最大索引值
func (s *localStorage) loadLastIndexes(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kvMax sql.NullInt64
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(idx) FROM kv_log").Scan(&kvMax); err != nil {
		return err
	}
	if kvMax.Valid {
		s.lastKVIndex = uint64(kvMax.Int64)
	}

	var raftMax sql.NullInt64
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(idx) FROM raft_log").Scan(&raftMax); err != nil {
		return err
	}
	if raftMax.Valid {
		s.lastRaftIndex = uint64(raftMax.Int64)
	}

	return nil
}

// 关闭存储
func (s *localStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// AppendLog 添加操作日志，返回日志索引
func (s *localStorage) AppendLog(ctx context.Context, cmd common.Command) (uint64, error) {
	select {
	case <-ctx.Done():
		return s.LastIndex(), ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return s.lastKVIndex, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		"INSERT INTO kv_log(op, k, v) VALUES(?, ?, ?)",
		string(cmd.Op), cmd.Key, cmd.Value,
	)
	if err != nil {
		return s.lastKVIndex, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return s.lastKVIndex, err
	}
	if err := tx.Commit(); err != nil {
		return s.lastKVIndex, err
	}

	if id > 0 {
		s.lastKVIndex = uint64(id)
	}
	return s.lastKVIndex, nil
}

// ApplyLog 应用指定索引的操作日志到状态机
func (s *localStorage) ApplyLog(ctx context.Context, index uint64) error {
	if index == 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var op, k, v string
	err := s.db.QueryRowContext(ctx, "SELECT op, k, v FROM kv_log WHERE idx = ?", int64(index)).Scan(&op, &k, &v)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	switch common.CommandOperation(op) {
	case common.OpPut:
		h := common.HashKey(k)
		_, err = s.db.ExecContext(ctx,
			"INSERT INTO kv_state(k, v, hash) VALUES(?, ?, ?) ON CONFLICT(k) DO UPDATE SET v=excluded.v, hash=excluded.hash",
			k, v, int64(h),
		)
		return err
	case common.OpDelete:
		_, err = s.db.ExecContext(ctx, "DELETE FROM kv_state WHERE k = ?", k)
		return err
	case common.OpNoop:
		return nil
	default:
		return nil
	}
}

// Get 获取指定键的值
func (s *localStorage) Get(ctx context.Context, key string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	var v string
	err := s.db.QueryRowContext(ctx, "SELECT v FROM kv_state WHERE k = ?", key).Scan(&v)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return v, nil
}

// LastIndex 返回当前最后一条业务日志的索引
func (s *localStorage) LastIndex() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastKVIndex
}

// BatchApply 批量应用操作日志到状态机
func (s *localStorage) BatchApply(ctx context.Context, cmds *[]common.Command) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if cmds == nil || len(*cmds) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, cmd := range *cmds {
		switch cmd.Op {
		case common.OpPut:
			h := common.HashKey(cmd.Key)
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO kv_state(k, v, hash) VALUES(?, ?, ?) ON CONFLICT(k) DO UPDATE SET v=excluded.v, hash=excluded.hash",
				cmd.Key, cmd.Value, int64(h),
			); err != nil {
				return err
			}
		case common.OpDelete:
			if _, err := tx.ExecContext(ctx, "DELETE FROM kv_state WHERE k = ?", cmd.Key); err != nil {
				return err
			}
		case common.OpNoop:
			// no-op
		}

		res, err := tx.ExecContext(ctx,
			"INSERT INTO kv_log(op, k, v) VALUES(?, ?, ?)",
			string(cmd.Op), cmd.Key, cmd.Value,
		)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if id > 0 {
			s.lastKVIndex = uint64(id)
		}
	}

	return tx.Commit()
}

// AppendBatchKV 批量添加业务数据，返回起始日志索引
func (s *localStorage) AppendBatchKV(ctx context.Context, kvs *[]common.KVPair) (uint64, error) {
	select {
	case <-ctx.Done():
		return s.LastIndex(), ctx.Err()
	default:
	}
	if kvs == nil || len(*kvs) == 0 {
		return s.LastIndex() + 1, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	startIndex := s.lastKVIndex + 1

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return startIndex, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, kv := range *kvs {
		h := common.HashKey(kv.Key)
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO kv_state(k, v, hash) VALUES(?, ?, ?) ON CONFLICT(k) DO UPDATE SET v=excluded.v, hash=excluded.hash",
			kv.Key, kv.Value, int64(h),
		); err != nil {
			return startIndex, err
		}
		res, err := tx.ExecContext(ctx,
			"INSERT INTO kv_log(op, k, v) VALUES(?, ?, ?)",
			string(common.OpPut), kv.Key, kv.Value,
		)
		if err != nil {
			return startIndex, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return startIndex, err
		}
		if id > 0 {
			s.lastKVIndex = uint64(id)
		}
	}

	if err := tx.Commit(); err != nil {
		return startIndex, err
	}
	return startIndex, nil
}

// GetBatch 批量读取 [startIndex, endIndex) 区间内的业务日志
func (s *localStorage) GetBatch(ctx context.Context, startIndex, endIndex uint64) (*[]common.Command, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if startIndex == 0 {
		startIndex = 1
	}
	if endIndex != 0 && startIndex > endIndex {
		return &[]common.Command{}, nil
	}
	if endIndex == 0 {
		endIndex = s.LastIndex() + 1
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT op, k, v FROM kv_log WHERE idx >= ? AND idx < ? ORDER BY idx ASC",
		int64(startIndex), int64(endIndex),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cmds := make([]common.Command, 0)
	for rows.Next() {
		var op, k, v string
		if err := rows.Scan(&op, &k, &v); err != nil {
			return nil, err
		}
		cmds = append(cmds, common.Command{Op: common.CommandOperation(op), Key: k, Value: v})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &cmds, nil
}

// GetHashRange 按 Key 哈希范围读取业务数据（不经过日志）
func (s *localStorage) GetHashRange(ctx context.Context, startHash, endHash uint32) (*[]common.KVPair, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if startHash == endHash {
		return &[]common.KVPair{}, nil
	}

	var query string
	var args []any
	if startHash < endHash {
		query = "SELECT k, v FROM kv_state WHERE hash >= ? AND hash < ?"
		args = []any{int64(startHash), int64(endHash)}
	} else {
		query = "SELECT k, v FROM kv_state WHERE hash >= ? OR hash < ?"
		args = []any{int64(startHash), int64(endHash)}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pairs := make([]common.KVPair, 0)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		pairs = append(pairs, common.KVPair{Key: k, Value: v})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &pairs, nil
}

// GetMoveRangeRecord 获取指定迁移 ID 的迁移记录是否存在
func (s *localStorage) GetMoveRangeRecord(ctx context.Context, moveID uint32) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	var exists int
	err := s.db.QueryRowContext(ctx,
		"SELECT 1 FROM move_range_records WHERE move_id = ? LIMIT 1",
		int64(moveID),
	).Scan(&exists)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SaveMoveRangeRecord 保存指定迁移 ID 的迁移记录
func (s *localStorage) SaveMoveRangeRecord(ctx context.Context, moveID uint32) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO move_range_records(move_id, created_at) VALUES(?, ?)",
		int64(moveID), time.Now().Format(time.RFC3339),
	)
	return err
}

// AppendRaftLog 追加一批 Raft 日志
func (s *localStorage) AppendRaftLog(ctx context.Context, entries []common.RaftLogEntry) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if len(entries) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	maxIdx := s.lastRaftIndex
	for _, e := range entries {
		var confJSON *string
		if e.Conf != nil {
			b, err := json.Marshal(e.Conf)
			if err != nil {
				return err
			}
			s := string(b)
			confJSON = &s
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT OR REPLACE INTO raft_log(idx, term, entry_type, cmd_op, cmd_k, cmd_v, conf_json) VALUES(?, ?, ?, ?, ?, ?, ?)",
			int64(e.Index), int64(e.Term), int64(e.Type), string(e.Cmd.Op), e.Cmd.Key, e.Cmd.Value, confJSON,
		); err != nil {
			return err
		}
		if e.Index > maxIdx {
			maxIdx = e.Index
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	s.lastRaftIndex = maxIdx
	return nil
}

// RaftLogEntries 读取 [from, to) 区间内的 Raft 日志
func (s *localStorage) RaftLogEntries(ctx context.Context, from, to uint64) ([]common.RaftLogEntry, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	if from == 0 {
		from = 1
	}
	if from > to {
		return []common.RaftLogEntry{}, nil
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT idx, term, entry_type, cmd_op, cmd_k, cmd_v, conf_json FROM raft_log WHERE idx >= ? AND idx < ? ORDER BY idx ASC",
		int64(from), int64(to),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]common.RaftLogEntry, 0)
	for rows.Next() {
		var idx, term int64
		var entryType int64
		var op, k, v string
		var conf sql.NullString
		if err := rows.Scan(&idx, &term, &entryType, &op, &k, &v, &conf); err != nil {
			return nil, err
		}
		var confObj *common.ClusterConfigChange
		if conf.Valid && conf.String != "" {
			var parsed common.ClusterConfigChange
			if err := json.Unmarshal([]byte(conf.String), &parsed); err != nil {
				return nil, err
			}
			confObj = &parsed
		}
		res = append(res, common.RaftLogEntry{
			Index: uint64(idx),
			Term:  uint64(term),
			Type:  common.LogEntryType(entryType),
			Cmd:   common.Command{Op: common.CommandOperation(op), Key: k, Value: v},
			Conf:  confObj,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

// RaftLogTerm 获取指定索引的 Raft 日志任期
func (s *localStorage) RaftLogTerm(ctx context.Context, index uint64) (uint64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	if index == 0 {
		return 0, nil
	}
	var term int64
	err := s.db.QueryRowContext(ctx, "SELECT term FROM raft_log WHERE idx = ?", int64(index)).Scan(&term)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return uint64(term), nil
}

// RaftLogLastIndex 返回当前 Raft 日志的最大索引
func (s *localStorage) RaftLogLastIndex(ctx context.Context) (uint64, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRaftIndex, nil
}

// RaftLogTruncateFrom 从 index 起（含）截断 Raft 日志
func (s *localStorage) RaftLogTruncateFrom(ctx context.Context, index uint64) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if index == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM raft_log WHERE idx >= ?", int64(index)); err != nil {
		return err
	}
	var max sql.NullInt64
	if err := tx.QueryRowContext(ctx, "SELECT MAX(idx) FROM raft_log").Scan(&max); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if max.Valid {
		s.lastRaftIndex = uint64(max.Int64)
	} else {
		s.lastRaftIndex = 0
	}
	return nil
}

// SaveRaftHardState 保存 Raft 硬状态
func (s *localStorage) SaveRaftHardState(ctx context.Context, hs common.RaftHardState) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO raft_hardstate(id, term, voted_for, commit_index) VALUES(1, ?, ?, ?) "+
			"ON CONFLICT(id) DO UPDATE SET term=excluded.term, voted_for=excluded.voted_for, commit_index=excluded.commit_index",
		int64(hs.Term), hs.VotedFor, int64(hs.CommitIndex),
	)
	return err
}

// LoadRaftHardState 读取 Raft 硬状态
func (s *localStorage) LoadRaftHardState(ctx context.Context) (common.RaftHardState, error) {
	select {
	case <-ctx.Done():
		return common.RaftHardState{}, ctx.Err()
	default:
	}
	var term, commit int64
	var voted string
	err := s.db.QueryRowContext(ctx, "SELECT term, voted_for, commit_index FROM raft_hardstate WHERE id = 1").Scan(&term, &voted, &commit)
	if err != nil {
		if err == sql.ErrNoRows {
			return common.RaftHardState{}, nil
		}
		return common.RaftHardState{}, err
	}
	return common.RaftHardState{Term: uint64(term), VotedFor: voted, CommitIndex: uint64(commit)}, nil
}

// 防止未来 schema 演进时静默失败
func (s *localStorage) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return "LocalStorage(sqlite: closed)"
	}
	return fmt.Sprintf("LocalStorage(sqlite: open, lastKV=%d, lastRaft=%d)", s.lastKVIndex, s.lastRaftIndex)
}
