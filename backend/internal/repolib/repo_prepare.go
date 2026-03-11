package repolib

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

type prepareSnapshotOutput struct {
	ResolvedRef   string `json:"resolved_ref,omitempty"`
	CommitSHA     string `json:"commit_sha,omitempty"`
	CodeIndexPath string `json:"code_index_path,omitempty"`
}

type codeIndex struct {
	EngineVersion string            `json:"engine_version"`
	GeneratedAt   string            `json:"generated_at"`
	RepoURL       string            `json:"repo_url"`
	RequestedRef  string            `json:"requested_ref"`
	ResolvedRef   string            `json:"resolved_ref,omitempty"`
	CommitSHA     string            `json:"commit_sha,omitempty"`
	FileCount     int               `json:"file_count"`
	Languages     map[string]int    `json:"languages,omitempty"`
	Entrypoints   []string          `json:"entrypoints,omitempty"`
	SampleFiles   []codeIndexSample `json:"sample_files,omitempty"`
}

type codeIndexSample struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// prepareSnapshotSource 功能：准备 snapshot/source 与最小 code_index.json。
// 参数/返回：repoURL/ref 指定 GitHub 仓库与 ref；sourceDir/artifactsDir 为输出目录；返回 resolvedRef/commitSHA/codeIndexPath。
// 失败场景：ZIP 与 git fallback 均失败、解压失败或写入失败时返回 error。
// 副作用：写入 sourceDir、artifactsDir/code_index.json。
func prepareSnapshotSource(ctx context.Context, repoURL, owner, repo, ref, sourceDir, artifactsDir string) (prepareSnapshotOutput, error) {
	if strings.TrimSpace(sourceDir) == "" || strings.TrimSpace(artifactsDir) == "" {
		return prepareSnapshotOutput{}, fmt.Errorf("prepare snapshot: sourceDir/artifactsDir are required")
	}
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		return prepareSnapshotOutput{}, err
	}
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		return prepareSnapshotOutput{}, err
	}

	if empty, err := dirEmpty(sourceDir); err != nil {
		return prepareSnapshotOutput{}, err
	} else if !empty {
		// Snapshot is intended to be immutable; refuse to overwrite.
		return prepareSnapshotOutput{}, fmt.Errorf("prepare snapshot: source dir is not empty: %s", sourceDir)
	}

	resolvedRef, commitSHA := resolveGitHubRef(ctx, owner, repo, ref)

	zipErr := fetchGitHubZip(ctx, owner, repo, firstNonEmpty(commitSHA, ref), sourceDir)
	if zipErr != nil {
		gitErr := fetchGitClone(ctx, repoURL, ref, sourceDir)
		if gitErr != nil {
			return prepareSnapshotOutput{}, fmt.Errorf("prepare snapshot: zip failed (%v); git fallback failed (%v)", zipErr, gitErr)
		}
	}

	codeIndexPath := filepath.Join(artifactsDir, "code_index.json")
	if err := writeMinimalCodeIndex(codeIndexPath, repoURL, ref, resolvedRef, commitSHA, sourceDir); err != nil {
		return prepareSnapshotOutput{}, err
	}

	return prepareSnapshotOutput{
		ResolvedRef:   firstNonEmpty(resolvedRef, ref),
		CommitSHA:     commitSHA,
		CodeIndexPath: codeIndexPath,
	}, nil
}

func dirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	entries, err := f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return len(entries) == 0, err
}

func resolveGitHubRef(ctx context.Context, owner, repo, ref string) (string, string) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	ref = strings.TrimSpace(ref)
	if owner == "" || repo == "" || ref == "" {
		return "", ""
	}
	// Best-effort: unauthenticated GitHub API calls are rate-limited; failure is acceptable.
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", owner, repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("User-Agent", "vibe-tree")
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", ""
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", ""
	}
	return ref, strings.TrimSpace(payload.SHA)
}

func fetchGitHubZip(ctx context.Context, owner, repo, ref, destDir string) error {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	ref = strings.TrimSpace(ref)
	if owner == "" || repo == "" || ref == "" {
		return fmt.Errorf("zip fetch: owner/repo/ref required")
	}
	// codeload supports /zip/<ref> where ref can be branch/tag/sha.
	url := fmt.Sprintf("https://codeload.github.com/%s/%s/zip/%s", owner, repo, ref)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "vibe-tree")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("zip fetch: status=%d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 512<<20))
	if err != nil {
		return err
	}
	return unzipStripRoot(raw, destDir)
}

func unzipStripRoot(raw []byte, destDir string) error {
	r, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return err
	}
	for _, f := range r.File {
		name := f.Name
		name = strings.TrimPrefix(name, "/")
		parts := strings.Split(name, "/")
		if len(parts) <= 1 {
			continue
		}
		rel := filepath.Join(parts[1:]...)
		rel = filepath.Clean(rel)
		if rel == "." || rel == string(filepath.Separator) || strings.HasPrefix(rel, "..") {
			continue
		}
		target := filepath.Join(destDir, rel)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		_ = out.Close()
		_ = rc.Close()
	}
	return nil
}

func fetchGitClone(ctx context.Context, repoURL, ref, destDir string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found")
	}
	// We clone into destDir directly; destDir is empty at this point.
	args := []string{"clone", "--depth=1"}
	if strings.TrimSpace(ref) != "" {
		args = append(args, "--branch", strings.TrimSpace(ref))
	}
	args = append(args, "--", strings.TrimSpace(repoURL), destDir)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func writeMinimalCodeIndex(path, repoURL, requestedRef, resolvedRef, commitSHA, sourceDir string) error {
	index := codeIndex{
		EngineVersion: "go-repo-prepare/v1",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		RepoURL:       strings.TrimSpace(repoURL),
		RequestedRef:  strings.TrimSpace(requestedRef),
		ResolvedRef:   strings.TrimSpace(resolvedRef),
		CommitSHA:     strings.TrimSpace(commitSHA),
		Languages:     map[string]int{},
	}
	samples := make([]codeIndexSample, 0, 64)
	entrypoints := make(map[string]struct{})

	_ = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			base := strings.ToLower(info.Name())
			if base == ".git" || base == "node_modules" || base == "vendor" || base == ".venv" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		index.FileCount++
		ext := strings.ToLower(filepath.Ext(rel))
		if ext != "" {
			index.Languages[ext]++
		}
		// Basic entrypoint heuristics.
		lower := strings.ToLower(rel)
		if lower == "main.go" || strings.HasSuffix(lower, "/main.go") ||
			lower == "package.json" || strings.HasSuffix(lower, "/package.json") ||
			lower == "pom.xml" || strings.HasSuffix(lower, "/pom.xml") ||
			lower == "cargo.toml" || strings.HasSuffix(lower, "/cargo.toml") ||
			lower == "pyproject.toml" || strings.HasSuffix(lower, "/pyproject.toml") ||
			lower == "readme.md" || strings.HasSuffix(lower, "/readme.md") {
			entrypoints[rel] = struct{}{}
		}
		if len(samples) < 120 {
			samples = append(samples, codeIndexSample{Path: rel, Size: info.Size()})
		}
		return nil
	})

	for k := range entrypoints {
		index.Entrypoints = append(index.Entrypoints, k)
	}
	sort.Strings(index.Entrypoints)
	index.SampleFiles = samples

	// Stable fingerprint to help debugging if needed.
	fingerprint := sha256.New()
	_, _ = fingerprint.Write([]byte(index.RepoURL))
	_, _ = fingerprint.Write([]byte(index.RequestedRef))
	_, _ = fingerprint.Write([]byte(index.CommitSHA))
	sum := hex.EncodeToString(fingerprint.Sum(nil))[:16]
	index.EngineVersion = index.EngineVersion + "+" + runtime.GOOS + "-" + runtime.GOARCH + "-" + sum

	payload, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}
