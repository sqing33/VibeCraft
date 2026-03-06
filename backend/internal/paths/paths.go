package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// DataDir 功能：返回数据目录路径（XDG data 优先）。
// 参数/返回：返回 `$XDG_DATA_HOME/vibe-tree`（或 `~/.local/share/vibe-tree`）。
// 失败场景：无法解析用户 home 目录时返回 error。
// 副作用：读取环境变量与 home 目录信息。
func DataDir() (string, error) {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		xdgDataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdgDataHome, "vibe-tree"), nil
}

// LogsDir 功能：返回日志目录路径（XDG data 下的 logs）。
// 参数/返回：返回 `$XDG_DATA_HOME/vibe-tree/logs`（或 `~/.local/share/vibe-tree/logs`）。
// 失败场景：无法解析用户 home 目录时返回 error。
// 副作用：读取环境变量与 home 目录信息。
func LogsDir() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "logs"), nil
}

// StateDBPath 功能：返回 SQLite state DB 路径。
// 参数/返回：返回 `$XDG_DATA_HOME/vibe-tree/state.db`（或 `~/.local/share/vibe-tree/state.db`）。
// 失败场景：dataDir 解析失败时返回 error。
// 副作用：读取环境变量与 home 目录信息（间接）。
func StateDBPath() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "state.db"), nil
}

// EnsureDir 功能：确保目录存在（幂等）。
// 参数/返回：传入目录路径；成功返回 nil。
// 失败场景：权限不足或路径不可创建时返回 error。
// 副作用：在磁盘上创建目录。
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

// ExecutionLogPath 功能：按 execution_id 生成对应日志文件路径。
// 参数/返回：返回 `<logsDir>/<execution_id>.log`。
// 失败场景：logsDir 解析失败时返回 error。
// 副作用：读取环境变量与 home 目录信息（间接）。
func ExecutionLogPath(executionID string) (string, error) {
	logsDir, err := LogsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(logsDir, fmt.Sprintf("%s.log", executionID)), nil
}

// ChatAttachmentsDir 功能：返回聊天附件根目录路径。
// 参数/返回：返回 `$XDG_DATA_HOME/vibe-tree/chat-attachments`（或 `~/.local/share/vibe-tree/chat-attachments`）。
// 失败场景：dataDir 解析失败时返回 error。
// 副作用：读取环境变量与 home 目录信息（间接）。
func ChatAttachmentsDir() (string, error) {
	dataDir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "chat-attachments"), nil
}

// ChatAttachmentMessageDir 功能：返回某条聊天消息的附件目录。
// 参数/返回：返回 `<chatAttachmentsDir>/<session_id>/<message_id>`。
// 失败场景：chatAttachmentsDir 解析失败时返回 error。
// 副作用：读取环境变量与 home 目录信息（间接）。
func ChatAttachmentMessageDir(sessionID, messageID string) (string, error) {
	root, err := ChatAttachmentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, sessionID, messageID), nil
}
