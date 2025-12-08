package store

// WAL 日志存储
type WALStore interface {
	// 追加一条日志记录
	Append(entry WALEntry) error
	// 读取从 startIndex 开始的最多 count 条日志记录
	ReadEntries(startIndex uint64, count int) ([]WALEntry, error)
	// 获取当前日志的最大索引
	GetMaxIndex() (uint64, error)
	// 关闭 WAL 存储
	Close() error
}

type WALEntry struct {
	Index uint64 // 日志索引
	Term  uint64 // 任期号
	Data  []byte // 日志数据
}
