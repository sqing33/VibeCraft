package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Save 功能：将 cfg 写回到 XDG config.json（原子替换 + 0600 权限）。
// 参数/返回：cfg 为完整配置；返回写入路径与错误信息。
// 失败场景：路径解析失败、序列化失败、写盘失败时返回 error。
// 副作用：在磁盘上创建/覆盖 `~/.config/vibe-tree/config.json`。
func Save(cfg Config) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	return path, SaveTo(path, cfg)
}

// SaveTo 功能：将 cfg 写回到指定路径（原子替换 + 0600 权限），便于测试。
// 参数/返回：path 为目标文件；cfg 为完整配置；成功返回 nil。
// 失败场景：创建目录/写盘/rename 失败时返回 error。
// 副作用：在磁盘上创建/覆盖目标文件。
func SaveTo(path string, cfg Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("ensure config dir: %w", err)
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	b = append(b, '\n')

	tmp, err := os.CreateTemp(dir, "config.json.*")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}

	tmpPath := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod temp config: %w", err)
	}

	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("sync temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("rename config: %w", err)
	}
	return nil
}

