package dotenv

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type Result struct {
	Enabled       bool
	Attempted     bool
	Loaded        bool
	Path          string
	Keys          int
	SkippedReason string // "disabled", "no_path", "no_repo_root"
	FailureReason string // "not_found", "read_error", "parse_error"
}

// Load 功能：在 daemon 启动阶段加载 dotenv，并将其写入进程环境变量。
//
// 规则：
// - 若 `VIBECRAFT_DOTENV=0`（兼容 `VIBE_TREE_DOTENV=0`）：跳过加载。
// - 若 `VIBECRAFT_DOTENV_PATH` 非空（兼容 `VIBE_TREE_DOTENV_PATH`）：从该路径加载。
// - 否则：向上查找 `.git`（目录或文件）定位 repo root，并尝试加载 `<repo_root>/.env`。
// - dotenv 写入环境变量采用覆盖策略（.env 覆盖同名 env）。
//
// 参数/返回：无入参；返回加载结果与错误（错误不会用于阻断启动，由调用方决定记录并继续）。
// 失败场景：读取/解析失败返回 error；文件不存在不视为 error（FailureReason="not_found"）。
// 副作用：可能覆盖进程环境变量（os.Setenv）。
func Load() (Result, error) {
	if firstEnv("VIBECRAFT_DOTENV", "VIBE_TREE_DOTENV") == "0" {
		return Result{Enabled: false, SkippedReason: "disabled"}, nil
	}

	path := strings.TrimSpace(firstEnv("VIBECRAFT_DOTENV_PATH", "VIBE_TREE_DOTENV_PATH"))
	if path == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Result{Enabled: true, SkippedReason: "no_path"}, err
		}
		if root, ok := findRepoRoot(wd); ok {
			path = filepath.Join(root, ".env")
		} else {
			return Result{Enabled: true, SkippedReason: "no_repo_root"}, nil
		}
	}

	m, err := godotenv.Read(path)
	if err != nil {
		if isNotExist(err) {
			return Result{
				Enabled:       true,
				Attempted:     true,
				Loaded:        false,
				Path:          path,
				FailureReason: "not_found",
			}, nil
		}
		return Result{
			Enabled:       true,
			Attempted:     true,
			Loaded:        false,
			Path:          path,
			FailureReason: classifyLoadError(err),
		}, err
	}

	for k, v := range m {
		// 覆盖同名 env（按约定：dotenv 优先）。
		_ = os.Setenv(k, v)
	}

	return Result{
		Enabled:   true,
		Attempted: true,
		Loaded:    true,
		Path:      path,
		Keys:      len(m),
	}, nil
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func findRepoRoot(start string) (string, bool) {
	dir := start
	if abs, err := filepath.Abs(start); err == nil {
		dir = abs
	}

	for {
		if fi, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			if fi.IsDir() || fi.Mode().IsRegular() {
				return dir, true
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	var pe *os.PathError
	if errors.As(err, &pe) && errors.Is(pe.Err, os.ErrNotExist) {
		return true
	}
	return false
}

func classifyLoadError(err error) string {
	if err == nil {
		return ""
	}
	// godotenv 在解析失败时通常返回 parse 错误；读取失败则多为 PathError。
	var pe *os.PathError
	if errors.As(err, &pe) {
		return "read_error"
	}
	return "parse_error"
}
