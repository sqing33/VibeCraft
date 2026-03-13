package execution

import (
	"fmt"
	"io"
	"os"

	"vibecraft/backend/internal/paths"
)

const maxTailBytes int64 = 5 * 1024 * 1024

// ReadLogTail 功能：读取指定 execution 的日志尾部（按字节数）。
// 参数/返回：tailBytes 为尾部字节数；返回 UTF-8 文本字节串与错误信息。
// 失败场景：日志文件不存在、无法读取或 seek 失败时返回 error。
// 副作用：读取磁盘文件。
func ReadLogTail(executionID string, tailBytes int64) ([]byte, error) {
	logPath, err := paths.ExecutionLogPath(executionID)
	if err != nil {
		return nil, err
	}
	return ReadTailFile(logPath, tailBytes)
}

// ReadTailFile 功能：读取任意文件的尾部内容（按字节数，带上限保护）。
// 参数/返回：path 为文件路径；tailBytes 为尾部字节数；返回字节串与错误信息。
// 失败场景：文件不存在、无法读取或 seek 失败时返回 error。
// 副作用：读取磁盘文件。
func ReadTailFile(path string, tailBytes int64) ([]byte, error) {
	if tailBytes < 0 {
		tailBytes = 0
	}
	if tailBytes > maxTailBytes {
		tailBytes = maxTailBytes
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	size := st.Size()
	start := size - tailBytes
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek: %w", err)
	}

	return io.ReadAll(f)
}
