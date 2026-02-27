package id

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// New 功能：生成一个带前缀的短随机 ID（用于 wf_/nd_/ex_ 等）。
// 参数/返回：prefix 为前缀字符串；返回形如 `ex_<hex>` 的 ID。
// 失败场景：加密随机失败时降级为基于时间戳的伪随机值。
// 副作用：读取系统随机源与当前时间。
func New(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		now := time.Now().UnixNano()
		return prefix + hex.EncodeToString([]byte{
			byte(now >> 56), byte(now >> 48), byte(now >> 40), byte(now >> 32),
			byte(now >> 24), byte(now >> 16), byte(now >> 8), byte(now),
		})
	}

	// 8 bytes == 16 hex chars; enough for dev/MVP without making IDs unwieldy.
	return prefix + hex.EncodeToString(b[:8])
}
