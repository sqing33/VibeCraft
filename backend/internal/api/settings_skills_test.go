package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"vibecraft/backend/internal/config"
)

func TestSkillSettings_GetReturnsDiscoveredCatalogWithEnabledState(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	skillPath := filepath.Join(home, ".codex", "skills", "my-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("name: my-skill\ndescription: local skill\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	cfg := config.Default()
	cfg.CLITools = []config.CLIToolConfig{{ID: "codex", Label: "Codex", ProtocolFamily: "openai", CLIFamily: "codex", Enabled: true}}
	if err := config.NormalizeCLITools(&cfg.CLITools, cfg.LLM); err != nil {
		t.Fatalf("normalize cli tools: %v", err)
	}
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	env := newTestEnv(t, cfg, 2)

	res, err := http.Get(env.httpSrv.URL + "/api/v1/settings/skills")
	if err != nil {
		t.Fatalf("get skill settings: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	var got struct {
		Skills []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Path        string `json:"path"`
			Source      string `json:"source"`
			Enabled     bool   `json:"enabled"`
		} `json:"skills"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	found := false
	for _, item := range got.Skills {
		if item.ID != "my-skill" {
			continue
		}
		found = true
		if item.Source == "" || item.Path == "" {
			t.Fatalf("expected source/path in response: %#v", item)
		}
		if !item.Enabled {
			t.Fatalf("expected skill to default enabled: %#v", item)
		}
	}
	if !found {
		t.Fatalf("expected my-skill in response: %#v", got.Skills)
	}
}

func TestSkillSettings_PutPersistsEnabledFlag(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	skillPath := filepath.Join(home, ".codex", "skills", "my-skill", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(skillPath, []byte("name: my-skill\ndescription: local skill\n"), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
	cfg := config.Default()
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	env := newTestEnv(t, cfg, 2)

	body, _ := json.Marshal(map[string]any{"skills": []map[string]any{{"id": "my-skill", "enabled": false}}})
	req, err := http.NewRequest(http.MethodPut, env.httpSrv.URL+"/api/v1/settings/skills", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new put request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put skill settings: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	var got struct {
		Skills []struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		} `json:"skills"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	found := false
	for _, item := range got.Skills {
		if item.ID != "my-skill" {
			continue
		}
		found = true
		if item.Enabled {
			t.Fatalf("expected my-skill to be disabled: %#v", got.Skills)
		}
	}
	if !found {
		t.Fatalf("expected my-skill in response: %#v", got.Skills)
	}
	persisted, _, err := config.LoadPersisted()
	if err != nil {
		t.Fatalf("load persisted: %v", err)
	}
	if len(persisted.SkillBindings) != 1 || persisted.SkillBindings[0].ID != "my-skill" || persisted.SkillBindings[0].Enabled {
		t.Fatalf("unexpected persisted bindings: %#v", persisted.SkillBindings)
	}
}

func TestSkillSettings_InstallFromZip(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	cfg := config.Default()
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	env := newTestEnv(t, cfg, 2)

	archive := buildSkillZip(t, map[string]string{
		"demo-skill/SKILL.md":  "name: demo-skill\ndescription: from zip\n",
		"demo-skill/notes.txt": "hello",
	})
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("archive", "demo-skill.zip")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(archive); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	res, err := http.Post(env.httpSrv.URL+"/api/v1/settings/skills/install", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("post install: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	installed := filepath.Join(home, ".codex", "skills", "demo-skill", "SKILL.md")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("expected installed skill: %v", err)
	}
}

func TestSkillSettings_InstallFromDirectoryUpload(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	cfg := config.Default()
	cfgPath, err := config.Path()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	if err := config.SaveTo(cfgPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	env := newTestEnv(t, cfg, 2)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	files := map[string]string{
		"folder-skill/SKILL.md":       "name: folder-skill\ndescription: from folder\n",
		"folder-skill/scripts/run.sh": "echo ok\n",
	}
	for name, content := range files {
		if err := writer.WriteField("paths", name); err != nil {
			t.Fatalf("write path field: %v", err)
		}
		part, err := writer.CreateFormFile("files", filepath.Base(name))
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatalf("write upload: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	res, err := http.Post(env.httpSrv.URL+"/api/v1/settings/skills/install", writer.FormDataContentType(), &body)
	if err != nil {
		t.Fatalf("post install: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %s", res.Status)
	}
	installed := filepath.Join(home, ".codex", "skills", "folder-skill", "scripts", "run.sh")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("expected installed skill file: %v", err)
	}
}

func buildSkillZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}
