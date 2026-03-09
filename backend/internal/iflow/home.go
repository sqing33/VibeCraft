package iflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vibe-tree/backend/internal/paths"
)

const (
	DefaultBaseURL = "https://apis.iflow.cn/v1"
	DefaultModel   = "glm-4.7"
)

type bootstrapSettings struct {
	CNA                    string `json:"cna,omitempty"`
	BootAnimationShown     bool   `json:"bootAnimationShown,omitempty"`
	HasViewedOfflineOutput bool   `json:"hasViewedOfflineOutput,omitempty"`
	Checkpointing          struct {
		Enabled bool `json:"enabled"`
	} `json:"checkpointing,omitempty"`
}

type GlobalSettings struct {
	SelectedAuthType string `json:"selectedAuthType,omitempty"`
	ModelName        string `json:"modelName,omitempty"`
	BaseURL          string `json:"baseUrl,omitempty"`
	APIKey           string `json:"apiKey,omitempty"`
	SearchAPIKey     string `json:"searchApiKey,omitempty"`
	IsServerOAuth2   string `json:"isServerOAuth2,omitempty"`
}

type BrowserAuthStatus struct {
	HomeDir          string `json:"home_dir,omitempty"`
	SettingsPath     string `json:"settings_path,omitempty"`
	Authenticated    bool   `json:"authenticated"`
	SelectedAuthType string `json:"selected_auth_type,omitempty"`
	ModelName        string `json:"model_name,omitempty"`
	BaseURL          string `json:"base_url,omitempty"`
	HasAPIKey        bool   `json:"has_api_key"`
}

// HomeDir 功能：返回 vibe-tree 管理的 iFlow HOME 根目录。
// 参数/返回：无入参；返回 `$XDG_DATA_HOME/vibe-tree/iflow-home`。
// 失败场景：数据目录解析失败时返回 error。
// 副作用：读取 XDG/用户目录环境。
func HomeDir() (string, error) {
	dataDir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, "iflow-home"), nil
}

// GlobalDir 功能：返回 managed HOME 下的 `.iflow` 目录。
// 参数/返回：无入参；返回 `<HomeDir>/.iflow`。
// 失败场景：HOME 目录解析失败时返回 error。
// 副作用：读取 XDG/用户目录环境。
func GlobalDir() (string, error) {
	homeDir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".iflow"), nil
}

// SettingsPath 功能：返回 managed iFlow settings.json 路径。
// 参数/返回：无入参；返回 `<HomeDir>/.iflow/settings.json`。
// 失败场景：HOME 目录解析失败时返回 error。
// 副作用：读取 XDG/用户目录环境。
func SettingsPath() (string, error) {
	globalDir, err := GlobalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(globalDir, "settings.json"), nil
}

// UserSettingsPath 功能：返回用户全局 iFlow settings.json 路径。
// 参数/返回：无入参；返回 `~/.iflow/settings.json`。
// 失败场景：HOME 目录解析失败时返回 error。
// 副作用：读取用户 home 环境。
func UserSettingsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".iflow", "settings.json"), nil
}

// EnsureHome 功能：确保 managed iFlow HOME、bootstrap settings 与可复用官方登录态可用。
// 参数/返回：无入参；成功返回 HOME 路径。
// 失败场景：目录或 settings 文件无法创建时返回 error。
// 副作用：创建目录/文件，并在需要时从用户全局 `~/.iflow/settings.json` 同步认证字段。
func EnsureHome() (string, error) {
	homeDir, err := HomeDir()
	if err != nil {
		return "", err
	}
	if err := paths.EnsureDir(homeDir); err != nil {
		return "", err
	}
	globalDir := filepath.Join(homeDir, ".iflow")
	if err := paths.EnsureDir(globalDir); err != nil {
		return "", err
	}
	settingsPath := filepath.Join(globalDir, "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		if err := syncManagedAuthFromUserHome(settingsPath); err != nil {
			return "", err
		}
		return homeDir, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	payload := bootstrapSettingsPayload()
	if err := writeJSON(settingsPath, payload); err != nil {
		return "", err
	}
	if err := syncManagedAuthFromUserHome(settingsPath); err != nil {
		return "", err
	}
	return homeDir, nil
}

// LoadGlobalSettings 功能：读取 managed iFlow settings.json 中的认证关键信息。
// 参数/返回：无入参；返回裁剪后的 GlobalSettings。
// 失败场景：文件读取/JSON 解析失败时返回 error；文件缺失返回零值与 nil。
// 副作用：读取磁盘文件。
func LoadGlobalSettings() (GlobalSettings, error) {
	settingsPath, err := SettingsPath()
	if err != nil {
		return GlobalSettings{}, err
	}
	return loadSettingsAtPath(settingsPath)
}

// DetectBrowserAuthStatus 功能：根据 managed iFlow settings.json 推断网页登录是否已完成。
// 参数/返回：无入参；返回 BrowserAuthStatus。
// 失败场景：目录解析失败或 settings 文件损坏时返回 error。
// 副作用：会先确保 managed HOME 存在，并在需要时复用用户全局 iFlow 登录态。
func DetectBrowserAuthStatus() (BrowserAuthStatus, error) {
	homeDir, err := EnsureHome()
	if err != nil {
		return BrowserAuthStatus{}, err
	}
	settingsPath, err := SettingsPath()
	if err != nil {
		return BrowserAuthStatus{}, err
	}
	settings, err := LoadGlobalSettings()
	if err != nil {
		return BrowserAuthStatus{}, err
	}
	authType := strings.TrimSpace(settings.SelectedAuthType)
	apiKey := strings.TrimSpace(settings.APIKey)
	return BrowserAuthStatus{
		HomeDir:          homeDir,
		SettingsPath:     settingsPath,
		Authenticated:    hasOfficialAuth(settings),
		SelectedAuthType: authType,
		ModelName:        strings.TrimSpace(settings.ModelName),
		BaseURL:          firstNonEmpty(strings.TrimSpace(settings.BaseURL), DefaultBaseURL),
		HasAPIKey:        apiKey != "",
	}, nil
}

func bootstrapSettingsPayload() bootstrapSettings {
	payload := bootstrapSettings{
		CNA:                    "vibe-tree",
		BootAnimationShown:     true,
		HasViewedOfflineOutput: true,
	}
	payload.Checkpointing.Enabled = true
	return payload
}

func loadSettingsAtPath(path string) (GlobalSettings, error) {
	raw, err := readJSONMap(path)
	if err != nil {
		if os.IsNotExist(err) {
			return GlobalSettings{}, nil
		}
		return GlobalSettings{}, err
	}
	return decodeGlobalSettings(raw), nil
}

func syncManagedAuthFromUserHome(managedSettingsPath string) error {
	managedRaw, err := readJSONMap(managedSettingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		managedRaw = map[string]any{}
	}
	managed := decodeGlobalSettings(managedRaw)
	if hasOfficialAuth(managed) {
		return nil
	}
	userSettingsPath, err := UserSettingsPath()
	if err != nil {
		return err
	}
	if filepath.Clean(userSettingsPath) == filepath.Clean(managedSettingsPath) {
		return nil
	}
	userRaw, err := readJSONMap(userSettingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	userSettings := decodeGlobalSettings(userRaw)
	if !hasOfficialAuth(userSettings) {
		return nil
	}
	if len(managedRaw) == 0 {
		payload := bootstrapSettingsPayload()
		bootstrapRaw, err := structToMap(payload)
		if err != nil {
			return err
		}
		managedRaw = bootstrapRaw
	}
	for _, key := range []string{"selectedAuthType", "modelName", "baseUrl", "apiKey", "searchApiKey", "isServerOAuth2"} {
		value := userRaw[key]
		if stringValue(value) == "" {
			continue
		}
		managedRaw[key] = value
	}
	if _, ok := managedRaw["bootAnimationShown"]; !ok {
		managedRaw["bootAnimationShown"] = true
	}
	if _, ok := managedRaw["hasViewedOfflineOutput"]; !ok {
		managedRaw["hasViewedOfflineOutput"] = true
	}
	if _, ok := managedRaw["checkpointing"]; !ok {
		managedRaw["checkpointing"] = map[string]any{"enabled": true}
	}
	return writeJSON(managedSettingsPath, managedRaw)
}

func hasOfficialAuth(settings GlobalSettings) bool {
	return strings.TrimSpace(settings.SelectedAuthType) == "iflow" && strings.TrimSpace(settings.APIKey) != ""
}

func decodeGlobalSettings(raw map[string]any) GlobalSettings {
	return GlobalSettings{
		SelectedAuthType: strings.TrimSpace(stringValue(raw["selectedAuthType"])),
		ModelName:        strings.TrimSpace(stringValue(raw["modelName"])),
		BaseURL:          strings.TrimSpace(stringValue(raw["baseUrl"])),
		APIKey:           strings.TrimSpace(stringValue(raw["apiKey"])),
		SearchAPIKey:     strings.TrimSpace(stringValue(raw["searchApiKey"])),
		IsServerOAuth2:   strings.TrimSpace(stringValue(raw["isServerOAuth2"])),
	}
}

func structToMap(value any) (map[string]any, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal bootstrap json: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("decode bootstrap json: %w", err)
	}
	return out, nil
}

func readJSONMap(path string) (map[string]any, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
