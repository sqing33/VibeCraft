package skillcatalog

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func UserInstallRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".codex", "skills"), nil
}

func InstallFromZip(header *multipart.FileHeader) (Entry, error) {
	if header == nil {
		return Entry{}, fmt.Errorf("zip file is required")
	}
	file, err := header.Open()
	if err != nil {
		return Entry{}, fmt.Errorf("open zip file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return Entry{}, fmt.Errorf("read zip file: %w", err)
	}
	workDir, err := os.MkdirTemp("", "skill-upload-zip-*")
	if err != nil {
		return Entry{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := extractZip(data, workDir); err != nil {
		return Entry{}, err
	}
	return installFromPreparedDir(workDir)
}

func InstallFromUploadedFiles(files []*multipart.FileHeader, relativePaths []string) (Entry, error) {
	if len(files) == 0 {
		return Entry{}, fmt.Errorf("at least one uploaded file is required")
	}
	workDir, err := os.MkdirTemp("", "skill-upload-dir-*")
	if err != nil {
		return Entry{}, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	for index, header := range files {
		if header == nil {
			continue
		}
		relPath := ""
		if index < len(relativePaths) {
			relPath = cleanUploadRelativePath(relativePaths[index])
		}
		if relPath == "" {
			relPath = cleanUploadRelativePath(header.Filename)
		}
		if relPath == "" {
			continue
		}
		targetPath := filepath.Join(workDir, relPath)
		if err := ensurePathWithin(workDir, targetPath); err != nil {
			return Entry{}, err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return Entry{}, fmt.Errorf("prepare upload dir: %w", err)
		}
		src, err := header.Open()
		if err != nil {
			return Entry{}, fmt.Errorf("open uploaded file %q: %w", header.Filename, err)
		}
		data, readErr := io.ReadAll(src)
		_ = src.Close()
		if readErr != nil {
			return Entry{}, fmt.Errorf("read uploaded file %q: %w", header.Filename, readErr)
		}
		if err := os.WriteFile(targetPath, data, 0o644); err != nil {
			return Entry{}, fmt.Errorf("write uploaded file %q: %w", header.Filename, err)
		}
	}

	return installFromPreparedDir(workDir)
}

func installFromPreparedDir(workDir string) (Entry, error) {
	entry, rootDir, err := locateSingleSkillRoot(workDir)
	if err != nil {
		return Entry{}, err
	}
	if strings.TrimSpace(entry.ID) == "" {
		return Entry{}, fmt.Errorf("skill id is required")
	}
	installRoot, err := UserInstallRoot()
	if err != nil {
		return Entry{}, err
	}
	if err := os.MkdirAll(installRoot, 0o755); err != nil {
		return Entry{}, fmt.Errorf("ensure skill install root: %w", err)
	}
	dstDir := filepath.Join(installRoot, entry.ID)
	stagingDir := dstDir + ".tmp"
	_ = os.RemoveAll(stagingDir)
	if err := copyDir(rootDir, stagingDir); err != nil {
		return Entry{}, err
	}
	if err := os.RemoveAll(dstDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return Entry{}, fmt.Errorf("remove existing skill dir: %w", err)
	}
	if err := os.Rename(stagingDir, dstDir); err != nil {
		_ = os.RemoveAll(stagingDir)
		return Entry{}, fmt.Errorf("activate installed skill: %w", err)
	}
	skillPath := filepath.Join(dstDir, "SKILL.md")
	installed := parseSkill(skillPath)
	if strings.TrimSpace(installed.ID) == "" {
		installed.ID = entry.ID
	}
	installed.Path = skillPath
	return installed, nil
}

func locateSingleSkillRoot(root string) (Entry, string, error) {
	matches := make([]string, 0, 2)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "SKILL.md" {
			matches = append(matches, path)
		}
		return nil
	})
	if len(matches) == 0 {
		return Entry{}, "", fmt.Errorf("uploaded content does not contain SKILL.md")
	}
	if len(matches) > 1 {
		sort.Strings(matches)
		return Entry{}, "", fmt.Errorf("uploaded content must contain exactly one SKILL.md")
	}
	skillPath := matches[0]
	entry := parseSkill(skillPath)
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = filepath.Base(filepath.Dir(skillPath))
	}
	return entry, filepath.Dir(skillPath), nil
}

func extractZip(data []byte, targetRoot string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("read zip archive: %w", err)
	}
	for _, file := range reader.File {
		relPath := cleanUploadRelativePath(file.Name)
		if relPath == "" {
			continue
		}
		targetPath := filepath.Join(targetRoot, relPath)
		if err := ensurePathWithin(targetRoot, targetPath); err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create zip dir: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create zip file dir: %w", err)
		}
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %q: %w", file.Name, err)
		}
		data, readErr := io.ReadAll(src)
		_ = src.Close()
		if readErr != nil {
			return fmt.Errorf("read zip entry %q: %w", file.Name, readErr)
		}
		if err := os.WriteFile(targetPath, data, file.Mode()); err != nil {
			return fmt.Errorf("write zip entry %q: %w", file.Name, err)
		}
	}
	return nil
}

func cleanUploadRelativePath(raw string) string {
	raw = filepath.ToSlash(strings.TrimSpace(raw))
	raw = strings.TrimPrefix(raw, "/")
	raw = strings.TrimPrefix(raw, "./")
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "/")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			return ""
		}
		cleaned = append(cleaned, part)
	}
	return filepath.Join(cleaned...)
}

func ensurePathWithin(root, candidate string) error {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return fmt.Errorf("resolve upload path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("uploaded path escapes target directory")
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode())
	})
}
