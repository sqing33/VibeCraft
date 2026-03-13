package searchdb

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const sqliteVecVersion = "0.1.6"

func (s *Service) ensureVecExtension(ctx context.Context) (string, error) {
	if s == nil {
		return "", fmt.Errorf("searchdb: not initialized")
	}
	if path := strings.TrimSpace(s.vecExtension); path != "" {
		return path, nil
	}

	baseDir := filepath.Join(filepath.Dir(s.dbPath), "sqlite-vec")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", err
	}

	assetName, libName, err := sqliteVecAssetFor(runtime.GOOS, runtime.GOARCH, sqliteVecVersion)
	if err != nil {
		return "", err
	}
	destPath := filepath.Join(baseDir, libName)
	if fileExists(destPath) {
		s.vecExtension = destPath
		return destPath, nil
	}

	if !envBool("VIBECRAFT_SQLITE_VEC_ALLOW_DOWNLOAD", true) {
		return "", fmt.Errorf("sqlite-vec missing at %s (set VIBECRAFT_SQLITE_VEC_PATH or VIBECRAFT_SQLITE_VEC_ALLOW_DOWNLOAD=1)", destPath)
	}

	url := fmt.Sprintf("https://github.com/asg017/sqlite-vec/releases/download/v%s/%s", sqliteVecVersion, assetName)
	if err := downloadAndExtractSingleFileTarGz(ctx, url, libName, destPath); err != nil {
		return "", err
	}
	s.vecExtension = destPath
	return destPath, nil
}

func sqliteVecAssetFor(goos, goarch, version string) (assetName string, libName string, err error) {
	switch goos {
	case "linux":
		switch goarch {
		case "amd64":
			return fmt.Sprintf("sqlite-vec-%s-loadable-linux-x86_64.tar.gz", version), "vec0.so", nil
		case "arm64":
			return fmt.Sprintf("sqlite-vec-%s-loadable-linux-aarch64.tar.gz", version), "vec0.so", nil
		}
	case "windows":
		switch goarch {
		case "amd64":
			return fmt.Sprintf("sqlite-vec-%s-loadable-windows-x86_64.tar.gz", version), "vec0.dll", nil
		}
	}
	return "", "", fmt.Errorf("sqlite-vec unsupported platform: %s/%s", goos, goarch)
}

func downloadAndExtractSingleFileTarGz(ctx context.Context, url string, expectedName string, destPath string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "vibecraft-searchdb")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download sqlite-vec: unexpected status %s", resp.Status)
	}

	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Base(strings.TrimSpace(h.Name))
		if name != expectedName {
			continue
		}
		if err := writeFileAtomic(destPath, tr, 0o755); err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("sqlite-vec archive missing %s", expectedName)
}

func writeFileAtomic(destPath string, r io.Reader, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	tmp := destPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, r)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return fallback
	}
}
