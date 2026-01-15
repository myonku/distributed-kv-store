package storage

import (
	"distributed-kv-store/configs"
	"distributed-kv-store/internal/errors"
	"fmt"
	"path/filepath"
	"strings"
)

// 返回新的内存存储实例的引用
func NewStorage(cfg configs.StorageConfig) (Storage, error) {
	mode := cfg.Mode
	if mode == "" {
		// 当配置了 path（或显式 sqlite_file）时默认走 sqlite，否则走 memory
		if cfg.Path != "" || cfg.SQLiteFile != "" {
			mode = configs.StorageModeSQLite
		} else {
			mode = configs.StorageModeMemory
		}
	}

	switch mode {
	case configs.StorageModeMemory:
		return newMemoryStorage(), nil
	case configs.StorageModeSQLite:
		return newLocalStorage(cfg)
	default:
		return nil, errors.Error{
			Type: errors.InvalidArgument,
			Info: fmt.Sprintf("unsupported storage mode: %s", mode),
		}
	}
}

// 解析 SQLite 数据库文件路径
func resolveSQLiteDBPath(cfg configs.StorageConfig) (string, error) {
	baseDir := cfg.BaseDir
	if baseDir == "" {
		baseDir = "."
	}

	pathOrDir := strings.TrimSpace(cfg.Path)
	if pathOrDir == "" {
		pathOrDir = baseDir
	}
	if !filepath.IsAbs(pathOrDir) {
		pathOrDir = filepath.Join(baseDir, pathOrDir)
	}
	pathOrDir = filepath.Clean(pathOrDir)

	lower := strings.ToLower(pathOrDir)
	if strings.HasSuffix(lower, ".db") || strings.HasSuffix(lower, ".sqlite") || strings.HasSuffix(lower, ".sqlite3") {
		return pathOrDir, nil
	}

	fileName := strings.TrimSpace(cfg.SQLiteFile)
	if fileName == "" {
		fileName = "data.db"
	}
	return filepath.Join(pathOrDir, fileName), nil
}
