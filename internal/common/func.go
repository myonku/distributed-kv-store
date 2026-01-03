package common

import "hash/crc32"

// 计算字符串的 CRC32 哈希值
func HashKey(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}

// 计算迁移动作的唯一 ID
func ComputeMoveID(epoch uint64, startHash, endHash uint32, fromID, toID string) uint32 {
	data := make([]byte, 0)
	data = append(data, []byte(fromID)...)
	data = append(data, []byte(toID)...)
	data = append(data, byte(startHash>>24), byte(startHash>>16), byte(startHash>>8), byte(startHash))
	data = append(data, byte(endHash>>24), byte(endHash>>16), byte(endHash>>8), byte(endHash))
	data = append(data, byte(epoch>>56), byte(epoch>>48), byte(epoch>>40), byte(epoch>>32))
	data = append(data, byte(epoch>>24), byte(epoch>>16), byte(epoch>>8), byte(epoch))
	return crc32.ChecksumIEEE(data)
}
