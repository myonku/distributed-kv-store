package common

import "hash/crc32"

// 计算字符串的 CRC32 哈希值
func HashKey(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}
