package skillcatalog

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Entry struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Path        string `json:"path,omitempty"`
}

// Discover 功能：扫描仓库与用户目录下的 SKILL.md，构建可供 expert builder 使用的技能目录。
// 参数/返回：无入参；返回按 id 排序去重后的技能条目。
// 失败场景：忽略单个目录/文件读取失败，返回已发现条目。
// 副作用：读取文件系统。
func Discover() []Entry {
	roots := candidateRoots()
	seen := make(map[string]Entry)
	for _, root := range roots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if d.Name() != "SKILL.md" {
				return nil
			}
			entry := parseSkill(path)
			if strings.TrimSpace(entry.ID) == "" {
				return nil
			}
			if _, ok := seen[entry.ID]; ok {
				return nil
			}
			seen[entry.ID] = entry
			return nil
		})
	}
	out := make([]Entry, 0, len(seen))
	for _, entry := range seen {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func candidateRoots() []string {
	roots := make([]string, 0, 6)
	seen := make(map[string]struct{})
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		roots = append(roots, path)
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
			add(filepath.Join(dir, ".codex", "skills"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".codex", "skills"))
		add(filepath.Join(home, ".cc-switch", "skills"))
	}
	return roots
}

func parseSkill(path string) Entry {
	file, err := os.Open(path)
	if err != nil {
		return Entry{}
	}
	defer file.Close()

	entry := Entry{Path: path}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "name:") && entry.ID == "" {
			entry.ID = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name:")), `"'`)
			continue
		}
		if strings.HasPrefix(line, "description:") && entry.Description == "" {
			entry.Description = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), `"'`)
		}
		if entry.ID != "" && entry.Description != "" {
			break
		}
	}
	if entry.ID == "" {
		entry.ID = filepath.Base(filepath.Dir(path))
	}
	return entry
}
