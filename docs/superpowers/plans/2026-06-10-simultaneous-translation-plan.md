# 同声传译桌面应用 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建一个 Windows 桌面应用，捕获系统音频，通过用户配置的多模态 LLM API 实时翻译，悬浮字幕显示。

**Architecture:** Go + Wails v2 桌面框架，Go 后端负责 WASAPI Loopback 音频捕获和 HTTP API 调用，Web 前端负责字幕悬浮窗和设置面板。

**Tech Stack:** Go 1.25, Wails v2, malgo (音频), HTML/CSS/JS (前端), Windows WASAPI

---

## 项目文件结构

```
trss/
├── main.go                  # Wails 入口
├── app.go                   # App 结构体, Wails 绑定方法
├── wails.json               # Wails 配置
├── go.mod / go.sum
├── audio/
│   └── capture.go           # WASAPI Loopback 音频捕获
├── translator/
│   ├── config.go            # 翻译配置结构体 + JSON 持久化
│   ├── config_test.go       # 配置测试
│   ├── client.go            # 通用多模态 HTTP 翻译客户端
│   └── client_test.go       # 客户端测试
├── frontend/
│   ├── index.html           # 主 HTML
│   ├── src/
│   │   ├── main.js          # 前端入口
│   │   ├── style.css        # 全局样式
│   │   ├── subtitle.js      # 字幕悬浮窗逻辑
│   │   └── settings.js      # 设置面板逻辑
│   └── dist/                # Wails 构建输出（自动生成）
└── build/
    └── appicon.png          # 应用图标
```

---

### Task 1: 安装开发环境

- [ ] **Step 1: 安装 GCC (mingw-w64)**

```bash
winget install -e --id GnuWin32.Make  # 或
# 推荐用 MSYS2: https://www.msys2.org/
```

验证:
```bash
gcc --version
# 预期: gcc (MinGW-W64 ...) 或 gcc (GCC) ...
```

- [ ] **Step 2: 安装 Wails CLI v2**

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

验证:
```bash
wails version
# 预期: v2.x.x
```

- [ ] **Step 3: 验证完整工具链**

```bash
wails doctor
# 预期: 所有检查项 ✓
```

---

### Task 2: 初始化 Wails 项目

- [ ] **Step 1: 生成项目骨架**

```bash
cd /d/trss
# 先备份现有文件
mkdir -p /tmp/trss-backup && cp -r docs /tmp/trss-backup/ 2>/dev/null

# 用 vanilla 模板初始化
wails init -n trss -t vanilla -g .
mv frontend/wailsjs frontend/src/ 2>/dev/null || true
```

- [ ] **Step 2: 恢复设计文档**

```bash
cp -r /tmp/trss-backup/docs . 2>/dev/null || true
```

- [ ] **Step 3: 调整项目结构，创建模块目录**

```bash
mkdir -p audio translator frontend/src
```

- [ ] **Step 4: 添加 Go 依赖**

```bash
go get github.com/gen2brain/malgo
go mod tidy
```

- [ ] **Step 5: 验证基础项目能编译**

```bash
wails build
# 预期: 编译成功，输出到 build/bin/
```

- [ ] **Step 6: 初始化 Git 并提交**

```bash
git add -A && git commit -m "feat: initialize Wails project scaffold"
```

---

### Task 3: 翻译配置管理 (TDD)

**Files:**
- Create: `translator/config.go`
- Create: `translator/config_test.go`

- [ ] **Step 1: 编写失败测试**

`translator/config_test.go`:
```go
package translator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewConfigStore(filepath.Join(tmpDir, "test_config.json"))

	cfg := Config{
		Name:       "英文→中文 (GPT-4o)",
		BaseURL:    "https://api.openai.com/v1",
		APIKey:     "sk-test-key",
		Model:      "gpt-4o-audio-preview",
		SourceLang: "en",
		TargetLang: "zh",
		Prompt:     "将以下{source}音频实时翻译成{target}，简洁自然。",
	}

	err := store.Save(cfg)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load("英文→中文 (GPT-4o)")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.BaseURL != cfg.BaseURL {
		t.Errorf("BaseURL: got %q, want %q", loaded.BaseURL, cfg.BaseURL)
	}
	if loaded.Model != cfg.Model {
		t.Errorf("Model: got %q, want %q", loaded.Model, cfg.Model)
	}
}

func TestConfigListAndDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewConfigStore(filepath.Join(tmpDir, "test_config.json"))

	store.Save(Config{Name: "Profile A"})
	store.Save(Config{Name: "Profile B"})

	profiles, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(profiles) != 2 {
		t.Errorf("List: got %d profiles, want 2", len(profiles))
	}

	err = store.Delete("Profile A")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	profiles, _ = store.List()
	if len(profiles) != 1 {
		t.Errorf("after delete: got %d profiles, want 1", len(profiles))
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid", Config{Name: "test", BaseURL: "https://api.openai.com", APIKey: "sk-xxx", Model: "gpt-4o"}, false},
		{"no name", Config{BaseURL: "https://api.openai.com", APIKey: "sk-xxx", Model: "gpt-4o"}, true},
		{"no url", Config{Name: "test", APIKey: "sk-xxx", Model: "gpt-4o"}, true},
		{"no key", Config{Name: "test", BaseURL: "https://api.openai.com", Model: "gpt-4o"}, true},
		{"no model", Config{Name: "test", BaseURL: "https://api.openai.com", APIKey: "sk-xxx"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./translator/ -v
# 预期: FAIL - undefined: Config, NewConfigStore, etc.
```

- [ ] **Step 3: 实现配置管理代码**

`translator/config.go`:
```go
package translator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// Config 表示一个翻译配置方案
type Config struct {
	Name       string `json:"name"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
	Prompt     string `json:"prompt"`
}

// Validate 检查必填字段
func (c *Config) Validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}
	if c.BaseURL == "" {
		return errors.New("base_url is required")
	}
	if c.APIKey == "" {
		return errors.New("api_key is required")
	}
	if c.Model == "" {
		return errors.New("model is required")
	}
	return nil
}

// ConfigStore 管理配置方案的持久化
type ConfigStore struct {
	filePath string
	mu       sync.RWMutex
}

// configFile 是存储文件的内部结构
type configFile struct {
	Profiles []Config `json:"profiles"`
}

// NewConfigStore 创建配置管理器
func NewConfigStore(filePath string) *ConfigStore {
	return &ConfigStore{filePath: filePath}
}

// Save 保存或更新一个配置方案
func (s *ConfigStore) Save(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cf, err := s.readAll()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	found := false
	for i, p := range cf.Profiles {
		if p.Name == cfg.Name {
			cf.Profiles[i] = cfg
			found = true
			break
		}
	}
	if !found {
		cf.Profiles = append(cf.Profiles, cfg)
	}

	return s.writeAll(cf)
}

// Load 按名称加载配置方案
func (s *ConfigStore) Load(name string) (*Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cf, err := s.readAll()
	if err != nil {
		return nil, err
	}
	for _, p := range cf.Profiles {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("profile %q not found", name)
}

// List 列出所有配置方案名称
func (s *ConfigStore) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cf, err := s.readAll()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, len(cf.Profiles))
	for i, p := range cf.Profiles {
		names[i] = p.Name
	}
	return names, nil
}

// Delete 删除指定配置方案
func (s *ConfigStore) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cf, err := s.readAll()
	if err != nil {
		return err
	}
	for i, p := range cf.Profiles {
		if p.Name == name {
			cf.Profiles = append(cf.Profiles[:i], cf.Profiles[i+1:]...)
			return s.writeAll(cf)
		}
	}
	return fmt.Errorf("profile %q not found", name)
}

func (s *ConfigStore) readAll() (configFile, error) {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return configFile{}, err
	}
	var cf configFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return configFile{}, err
	}
	return cf, nil
}

func (s *ConfigStore) writeAll(cf configFile) error {
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0644)
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./translator/ -v
# 预期: PASS - all tests pass
```

- [ ] **Step 5: 提交**

```bash
git add translator/ go.mod go.sum
git commit -m "feat: add translation config management with JSON persistence"
```

---

### Task 4: 翻译客户端 (TDD)

**Files:**
- Create: `translator/client.go`
- Create: `translator/client_test.go`

- [ ] **Step 1: 编写失败测试**

`translator/client_test.go`:
```go
package translator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSendAudio(t *testing.T) {
	// Mock 多模态 API 服务
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求格式为 OpenAI Chat Completions 兼容
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("missing auth header")
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("wrong content-type: %s", ct)
		}
		// 返回翻译结果
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": "你好，这是一条测试翻译。",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Name:       "test",
		BaseURL:    server.URL,
		APIKey:     "sk-test",
		Model:      "gpt-4o",
		SourceLang: "en",
		TargetLang: "zh",
		Prompt:     "将{source}翻译为{target}",
	})

	result, err := client.Translate([]byte("fake-audio-data"))
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}
	if result.Text != "你好，这是一条测试翻译。" {
		t.Errorf("Text: got %q, want %q", result.Text, "你好，这是一条测试翻译。")
	}
}

func TestClientTestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]interface{}{"content": "ok"}},
			},
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		Name:    "test",
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "gpt-4o",
	})

	err := client.TestConnection()
	if err != nil {
		t.Fatalf("TestConnection failed: %v", err)
	}
}

func TestClientBuildPrompt(t *testing.T) {
	client := NewClient(Config{
		SourceLang: "en",
		TargetLang: "zh",
		Prompt:     "将{source}翻译为{target}，保持简洁。",
	})
	got := client.buildSystemPrompt()
	want := "将en翻译为zh，保持简洁。"
	if got != want {
		t.Errorf("prompt: got %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./translator/ -run "TestClient" -v
# 预期: FAIL - undefined: NewClient
```

- [ ] **Step 3: 实现翻译客户端**

`translator/client.go`:
```go
package translator

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client 通用多模态翻译客户端
type Client struct {
	cfg    Config
	client *http.Client
}

// Result 翻译结果
type Result struct {
	Text   string `json:"text"`
	IsFinal bool  `json:"is_final"`
}

// NewClient 创建翻译客户端
func NewClient(cfg Config) *Client {
	return &Client{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// buildSystemPrompt 用配置变量替换提示词中的占位符
func (c *Client) buildSystemPrompt() string {
	s := c.cfg.Prompt
	s = strings.ReplaceAll(s, "{source}", c.cfg.SourceLang)
	s = strings.ReplaceAll(s, "{target}", c.cfg.TargetLang)
	return s
}

// Translate 发送音频数据到多模态 API，返回翻译文本
// 音频以 base64 编码嵌入请求，使用 OpenAI Chat Completions 兼容格式
func (c *Client) Translate(audio []byte) (*Result, error) {
	audioB64 := base64.StdEncoding.EncodeToString(audio)

	reqBody := map[string]interface{}{
		"model": c.cfg.Model,
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": c.buildSystemPrompt(),
			},

			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "input_audio",
						"input_audio": map[string]interface{}{
							"data":   audioB64,
							"format": "wav",
						},
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	return &Result{
		Text:    chatResp.Choices[0].Message.Content,
		IsFinal: true,
	}, nil
}

// TestConnection 发送一个轻量请求测试 API 连通性
func (c *Client) TestConnection() error {
	reqBody := map[string]interface{}{
		"model": c.cfg.Model,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "ping"},
		},
		"max_tokens": 1,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	url := strings.TrimRight(c.cfg.BaseURL, "/") + "/chat/completions"
	req, _ := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error: %d", resp.StatusCode)
	}
	// 401/403 也是连通，只是认证问题
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./translator/ -v
# 预期: PASS - all tests pass (包括 config 和 client)
```

- [ ] **Step 5: 提交**

```bash
git add translator/
git commit -m "feat: add universal multimodal translation client"
```

---

### Task 5: 音频捕获模块

**Files:**
- Create: `audio/capture.go`

> 注：音频捕获依赖硬件，不适用标准单元测试。此模块通过 Wails 集成测试验证。

- [ ] **Step 1: 实现 WASAPI Loopback 音频捕获**

`audio/capture.go`:
```go
package audio

import (
	"fmt"
	"sync"

	"github.com/gen2brain/malgo"
)

// Config 音频捕获配置
type Config struct {
	SampleRate    int // 默认 16000
	Channels      int // 默认 1 (mono)
	FrameDuration int // 每帧毫秒数，默认 100
}

// DefaultConfig 返回默认音频配置
func DefaultConfig() Config {
	return Config{
		SampleRate:    16000,
		Channels:      1,
		FrameDuration: 100,
	}
}

// Frame 一帧音频数据
type Frame struct {
	Data       []byte
	Timestamp  int64 // 毫秒时间戳
	SampleRate int
}

// Capture 系统音频捕获器
type Capture struct {
	cfg    Config
	device *malgo.Device
	ctx    *malgo.AllocatedContext

	mu     sync.Mutex
	buffer [][]byte // 环形缓冲区
	running bool
	onFrame func(Frame)
}

// NewCapture 创建音频捕获器
func NewCapture(cfg Config) (*Capture, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("init audio context: %w", err)
	}

	c := &Capture{
		cfg: cfg,
		ctx: ctx,
	}

	return c, nil
}

// SetFrameCallback 设置每帧回调
func (c *Capture) SetFrameCallback(fn func(Frame)) {
	c.mu.Lock()
	c.onFrame = fn
	c.mu.Unlock()
}

// Start 开始捕获系统音频
func (c *Capture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = uint32(c.cfg.Channels)
	deviceConfig.SampleRate = uint32(c.cfg.SampleRate)
	deviceConfig.PeriodSizeInMilliseconds = uint32(c.cfg.FrameDuration)

	// 使用 WASAPI loopback 设备
	deviceConfig.Wasapi.NoAutoConvertSRC = true

	onRecvFrames := func(outputSample, inputSamples []byte, framecount uint32) {
		c.mu.Lock()
		fn := c.onFrame
		c.mu.Unlock()

		if fn != nil {
			fn(Frame{
				Data:       inputSamples,
				SampleRate: c.cfg.SampleRate,
			})
		}
	}

	var err error
	c.device, err = malgo.InitDevice(c.ctx.Context, deviceConfig, malgo.DeviceCallbacks{
		Data: onRecvFrames,
		Stop: func() {
			c.mu.Lock()
			c.running = false
			c.mu.Unlock()
		},
	})
	if err != nil {
		return fmt.Errorf("init capture device: %w", err)
	}

	err = c.device.Start()
	if err != nil {
		c.device.Uninit()
		return fmt.Errorf("start capture: %w", err)
	}

	c.running = true
	return nil
}

// Stop 停止音频捕获
func (c *Capture) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return nil
	}

	if c.device != nil {
		c.device.Stop()
		c.device.Uninit()
	}
	c.running = false
	return nil
}

// IsRunning 返回是否正在捕获
func (c *Capture) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.running
}

// Close 释放资源
func (c *Capture) Close() error {
	if err := c.Stop(); err != nil {
		return err
	}
	if c.ctx != nil {
		c.ctx.Uninit()
		c.ctx.Free()
	}
	return nil
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./audio/
# 预期: 编译成功
```

- [ ] **Step 3: 提交**

```bash
git add audio/
git commit -m "feat: add WASAPI loopback audio capture module"
```

---

### Task 6: Go 后端集成 — App 结构体

**Files:**
- Create: `app.go`

- [ ] **Step 1: 创建 App 结构体，暴露 Wails 绑定方法**

`app.go`:
```go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"trss/audio"
	"trss/translator"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 应用主结构体，方法自动暴露给前端
type App struct {
	ctx        context.Context
	capture    *audio.Capture
	translator *translator.Client
	configs    *translator.ConfigStore
	isListening bool
}

// NewApp 创建应用实例
func NewApp() *App {
	configDir, _ := os.UserConfigDir()
	configPath := filepath.Join(configDir, "trss", "configs.json")
	os.MkdirAll(filepath.Dir(configPath), 0755)

	return &App{
		configs: translator.NewConfigStore(configPath),
	}
}

// Startup 在 Wails 启动时调用
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// === 配置管理（暴露给前端）===

// GetConfigs 返回所有配置方案列表
func (a *App) GetConfigs() ([]translator.Config, error) {
	names, err := a.configs.List()
	if err != nil {
		return nil, err
	}
	var configs []translator.Config
	for _, name := range names {
		cfg, err := a.configs.Load(name)
		if err != nil {
			continue
		}
		// 不暴露 API Key 给前端
		cfg.APIKey = "••••••••"
		configs = append(configs, *cfg)
	}
	return configs, nil
}

// SaveConfig 保存配置方案
func (a *App) SaveConfig(cfg translator.Config) error {
	return a.configs.Save(cfg)
}

// DeleteConfig 删除配置方案
func (a *App) DeleteConfig(name string) error {
	return a.configs.Delete(name)
}

// TestConnection 测试当前配置的 API 连通性
func (a *App) TestConnection(cfgName string) error {
	cfg, err := a.configs.Load(cfgName)
	if err != nil {
		return err
	}
	client := translator.NewClient(*cfg)
	return client.TestConnection()
}

// === 翻译控制 ===

// StartListening 开始监听并翻译
func (a *App) StartListening(cfgName string) error {
	cfg, err := a.configs.Load(cfgName)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 初始化音频捕获
	capture, err := audio.NewCapture(audio.DefaultConfig())
	if err != nil {
		return fmt.Errorf("init audio: %w", err)
	}

	a.translator = translator.NewClient(*cfg)
	a.capture = capture

	// 设置音频帧回调
	capture.SetFrameCallback(func(frame audio.Frame) {
		if !a.isListening {
			return
		}
		// 异步发送翻译请求
		go func() {
			result, err := a.translator.Translate(frame.Data)
			if err != nil {
				runtime.EventsEmit(a.ctx, "error", err.Error())
				return
			}
			if result != nil {
				runtime.EventsEmit(a.ctx, "subtitle", result)
			}
		}()
	})

	a.isListening = true
	return capture.Start()
}

// StopListening 停止监听
func (a *App) StopListening() {
	a.isListening = false
	if a.capture != nil {
		a.capture.Stop()
	}
}

// PauseListening 暂停/恢复监听
func (a *App) PauseListening() {
	a.isListening = !a.isListening
	runtime.EventsEmit(a.ctx, "status", map[string]bool{"listening": a.isListening})
}
```

- [ ] **Step 2: 更新 main.go**

`main.go`:
```go
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "同声传译",
		Width:     400,
		Height:    500,
		MinWidth:  300,
		MinHeight: 400,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:        app.Startup,
		Bind:             []interface{}{app},
		WindowStartState: windows.Normal,
		Frameless:        false,
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
```

- [ ] **Step 3: 验证编译**

```bash
go build -o /dev/null .
# 预期: 编译成功
```

- [ ] **Step 4: 提交**

```bash
git add app.go main.go
git commit -m "feat: add App struct with Wails bindings for config and translation control"
```

---

### Task 7: 前端 — 字幕悬浮窗

**Files:**
- Modify: `frontend/index.html`
- Create: `frontend/src/subtitle.js`
- Modify: `frontend/src/style.css`
- Create: `frontend/src/main.js`

- [ ] **Step 1: 创建字幕悬浮窗 HTML 结构和样式**

`frontend/index.html`:
```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>同声传译</title>
  <link rel="stylesheet" href="/src/style.css" />
</head>
<body>
  <!-- 主控制栏 -->
  <div id="toolbar">
    <select id="profile-select">
      <option value="">选择配置方案...</option>
    </select>
    <button id="btn-toggle">▶ 开始</button>
    <button id="btn-settings">⚙</button>
  </div>

  <!-- 字幕悬浮区 -->
  <div id="subtitle-container">
    <div id="subtitle-source" class="subtitle-line src hidden"></div>
    <div id="subtitle-target" class="subtitle-line tgt"></div>
  </div>

  <!-- 状态栏 -->
  <div id="status-bar">
    <span id="status-text">● 就绪</span>
    <select id="display-mode">
      <option value="target">仅翻译</option>
      <option value="bilingual">双语</option>
      <option value="source">仅原文</option>
    </select>
  </div>

  <script src="/src/main.js"></script>
  <script src="/src/subtitle.js"></script>
  <script src="/src/settings.js"></script>
</body>
</html>
```

`frontend/src/style.css`:
```css
:root {
  --bg-dark: rgba(0, 0, 0, 0.85);
  --text-light: #ffffff;
  --accent: #4fc3f7;
  --radius: 8px;
}

* { margin: 0; padding: 0; box-sizing: border-box; }

body {
  font-family: "Microsoft YaHei", "PingFang SC", sans-serif;
  background: transparent;
  overflow: hidden;
  user-select: none;
}

#toolbar {
  position: fixed;
  top: 12px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 8px 16px;
  background: var(--bg-dark);
  border-radius: 20px;
  z-index: 1000;
}

#toolbar button {
  padding: 6px 14px;
  border: 1px solid rgba(255,255,255,0.2);
  border-radius: 14px;
  background: transparent;
  color: var(--text-light);
  cursor: pointer;
  font-size: 14px;
  transition: background 0.2s;
}
#toolbar button:hover { background: rgba(255,255,255,0.15); }
#toolbar button.active { background: var(--accent); color: #000; }

#toolbar select {
  padding: 4px 8px;
  border-radius: 10px;
  border: 1px solid rgba(255,255,255,0.2);
  background: transparent;
  color: var(--text-light);
  font-size: 13px;
}

#subtitle-container {
  position: fixed;
  bottom: 60px;
  left: 50%;
  transform: translateX(-50%);
  max-width: 90vw;
  text-align: center;
  z-index: 999;
}

.subtitle-line {
  padding: 10px 24px;
  border-radius: var(--radius);
  background: var(--bg-dark);
  color: var(--text-light);
  font-size: 20px;
  line-height: 1.5;
  margin: 4px 0;
  transition: opacity 0.3s;
  display: inline-block;
}

.subtitle-line.src { font-size: 16px; opacity: 0.8; }
.subtitle-line.tgt { font-size: 22px; font-weight: 600; }
.subtitle-line.hidden { display: none; }
.subtitle-line.final { border-left: 3px solid var(--accent); }

#status-bar {
  position: fixed;
  bottom: 10px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 4px 12px;
  background: var(--bg-dark);
  border-radius: 12px;
  z-index: 1000;
  font-size: 12px;
}

#status-text { color: #888; }
#status-text.active { color: #4caf50; }

#display-mode {
  padding: 2px 6px;
  border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.15);
  background: transparent;
  color: #aaa;
  font-size: 11px;
}
```

- [ ] **Step 2: 创建字幕显示逻辑**

`frontend/src/subtitle.js`:
```javascript
// 字幕显示管理
const Subtitle = {
  mode: 'target', // 'target' | 'bilingual' | 'source'
  timeoutId: null,

  init() {
    this.sourceEl = document.getElementById('subtitle-source');
    this.targetEl = document.getElementById('subtitle-target');
    this.modeSel = document.getElementById('display-mode');

    this.modeSel.addEventListener('change', () => {
      this.setMode(this.modeSel.value);
    });

    // 监听后端事件
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.EventsOn('subtitle', (result) => {
        this.show(result.text, result.isFinal);
      });

      window.go.main.App.EventsOn('error', (msg) => {
        console.error('[TRSS]', msg);
      });
    }
  },

  setMode(mode) {
    this.mode = mode;
    if (mode === 'source') {
      this.targetEl.classList.add('hidden');
      this.sourceEl.classList.remove('hidden');
    } else if (mode === 'target') {
      this.targetEl.classList.remove('hidden');
      this.sourceEl.classList.add('hidden');
    } else {
      this.targetEl.classList.remove('hidden');
      this.sourceEl.classList.remove('hidden');
    }
  },

  show(text, isFinal) {
    clearTimeout(this.timeoutId);

    if (this.mode !== 'source') {
      this.targetEl.textContent = text;
      this.targetEl.classList.toggle('final', isFinal);
    }
    if (this.mode === 'bilingual' || this.mode === 'source') {
      this.sourceEl.textContent = text;
    }

    if (isFinal) {
      // 最终结果停留 5 秒后淡出
      this.timeoutId = setTimeout(() => {
        this.targetEl.style.opacity = '0.3';
      }, 5000);
    } else {
      this.targetEl.style.opacity = '1';
    }
  }
};
```

- [ ] **Step 3: 创建前端入口**

`frontend/src/main.js`:
```javascript
document.addEventListener('DOMContentLoaded', () => {
  Subtitle.init();
  Settings.init();

  const btnToggle = document.getElementById('btn-toggle');
  const profileSelect = document.getElementById('profile-select');
  const statusText = document.getElementById('status-text');

  let isRunning = false;

  // 加载配置方案列表
  async function loadProfiles() {
    if (!window.go || !window.go.main || !window.go.main.App) return;
    try {
      const configs = await window.go.main.App.GetConfigs();
      profileSelect.innerHTML = '<option value="">选择配置方案...</option>';
      (configs || []).forEach((cfg) => {
        const opt = document.createElement('option');
        opt.value = cfg.name;
        opt.textContent = cfg.name;
        profileSelect.appendChild(opt);
      });
    } catch (e) {
      console.error('Failed to load profiles:', e);
    }
  }

  btnToggle.addEventListener('click', async () => {
    const profile = profileSelect.value;
    if (!profile) {
      alert('请先选择配置方案');
      return;
    }

    if (!isRunning) {
      try {
        await window.go.main.App.StartListening(profile);
        isRunning = true;
        btnToggle.textContent = '⏸ 暂停';
        btnToggle.classList.add('active');
        statusText.textContent = '● 运行中';
        statusText.classList.add('active');
      } catch (e) {
        alert('启动失败: ' + e);
      }
    } else {
      window.go.main.App.PauseListening();
      isRunning = false;
      btnToggle.textContent = '▶ 开始';
      btnToggle.classList.remove('active');
      statusText.textContent = '⏸ 已暂停';
      statusText.classList.remove('active');
    }
  });

  // 初始加载
  loadProfiles();
  setInterval(loadProfiles, 3000); // 定期刷新
});
```

- [ ] **Step 4: 提交**

```bash
git add frontend/
git commit -m "feat: add subtitle overlay frontend with display modes"
```

---

### Task 8: 前端 — 设置面板

**Files:**
- Create: `frontend/src/settings.js`
- Modify: `frontend/index.html`

- [ ] **Step 1: 创建设置面板逻辑**

`frontend/src/settings.js`:
```javascript
const Settings = {
  init() {
    document.getElementById('btn-settings').addEventListener('click', () => {
      this.toggle();
    });
    this.createPanel();
  },

  createPanel() {
    const panel = document.createElement('div');
    panel.id = 'settings-panel';
    panel.innerHTML = `
      <div class="settings-overlay"></div>
      <div class="settings-dialog">
        <div class="settings-header">
          <h2>设置</h2>
          <button id="btn-close-settings">✕</button>
        </div>
        <div class="settings-tabs">
          <button class="tab active" data-tab="translation">翻译配置</button>
          <button class="tab" data-tab="display">显示</button>
        </div>

        <div class="tab-content" id="tab-translation">
          <div class="form-group">
            <label>方案名称</label>
            <input type="text" id="cfg-name" placeholder="如：英文→中文 (GPT-4o)" />
          </div>
          <div class="form-group">
            <label>API 地址</label>
            <input type="text" id="cfg-url" placeholder="https://api.openai.com/v1" />
          </div>
          <div class="form-group">
            <label>API Key</label>
            <input type="password" id="cfg-key" placeholder="sk-..." />
          </div>
          <div class="form-group">
            <label>模型名称</label>
            <input type="text" id="cfg-model" placeholder="gpt-4o-audio-preview" />
          </div>
          <div class="form-row">
            <div class="form-group">
              <label>源语言</label>
              <select id="cfg-source"><option>en</option><option>zh</option><option>ja</option><option>ko</option></select>
            </div>
            <div class="form-group">
              <label>目标语言</label>
              <select id="cfg-target"><option>zh</option><option>en</option><option>ja</option><option>ko</option></select>
            </div>
          </div>
          <div class="form-group">
            <label>系统提示词（可用变量: {source} {target}）</label>
            <textarea id="cfg-prompt" rows="4">将{source}实时翻译为{target}。要求简洁自然，适合字幕阅读。保留原意，不添加解释。每次只输出翻译后的一句话。</textarea>
          </div>
          <div class="btn-row">
            <button id="btn-test">测试连接</button>
            <button id="btn-save" class="primary">保存方案</button>
          </div>
          <div id="test-result"></div>
        </div>

        <div class="tab-content hidden" id="tab-display">
          <div class="form-group">
            <label>字体大小</label>
            <input type="range" id="dsp-font-size" min="14" max="40" value="22" />
          </div>
          <div class="form-group">
            <label>背景透明度</label>
            <input type="range" id="dsp-bg-opacity" min="30" max="100" value="85" />
          </div>
        </div>
      </div>
    `;
    document.body.appendChild(panel);

    // 事件绑定
    this.bindEvents(panel);
  },

  bindEvents(panel) {
    const toggle = () => {
      panel.classList.toggle('open');
      if (panel.classList.contains('open')) this.loadProfiles();
    };

    panel.querySelector('#btn-close-settings').addEventListener('click', toggle);
    panel.querySelector('.settings-overlay').addEventListener('click', toggle);

    // 标签切换
    panel.querySelectorAll('.tab').forEach(tab => {
      tab.addEventListener('click', () => {
        panel.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
        tab.classList.add('active');
        panel.querySelectorAll('.tab-content').forEach(c => c.classList.add('hidden'));
        panel.querySelector(`#tab-${tab.dataset.tab}`).classList.remove('hidden');
      });
    });

    // 测试连接
    panel.querySelector('#btn-test').addEventListener('click', async () => {
      const name = panel.querySelector('#cfg-name').value;
      const result = panel.querySelector('#test-result');
      result.textContent = '测试中...';
      try {
        await window.go.main.App.TestConnection(name);
        result.innerHTML = '<span style="color:#4caf50">✓ 连接成功</span>';
      } catch (e) {
        result.innerHTML = `<span style="color:#f44336">✗ 失败: ${e}</span>`;
      }
    });

    // 保存配置
    panel.querySelector('#btn-save').addEventListener('click', async () => {
      const cfg = {
        name: panel.querySelector('#cfg-name').value,
        base_url: panel.querySelector('#cfg-url').value,
        api_key: panel.querySelector('#cfg-key').value,
        model: panel.querySelector('#cfg-model').value,
        source_lang: panel.querySelector('#cfg-source').value,
        target_lang: panel.querySelector('#cfg-target').value,
        prompt: panel.querySelector('#cfg-prompt').value,
      };
      try {
        await window.go.main.App.SaveConfig(cfg);
        toggle();
      } catch (e) {
        alert('保存失败: ' + e);
      }
    });
  },

  toggle() {
    document.getElementById('settings-panel').classList.toggle('open');
  },

  async loadProfiles() {
    try {
      const configs = await window.go.main.App.GetConfigs();
      // 填充配置列表
    } catch (e) {
      console.error(e);
    }
  }
};
```

- [ ] **Step 2: 添加设置面板样式**

在 `frontend/src/style.css` 末尾追加：
```css
.settings-overlay {
  position: fixed; inset: 0;
  background: rgba(0,0,0,0.5); z-index: 2000;
}
.settings-dialog {
  position: fixed; top: 50%; left: 50%;
  transform: translate(-50%, -50%);
  width: 520px; max-height: 80vh;
  background: #1e1e2e; border-radius: 16px;
  z-index: 2001; overflow-y: auto;
  color: #cdd6f4;
}
.settings-header {
  display: flex; justify-content: space-between;
  align-items: center; padding: 16px 20px;
  border-bottom: 1px solid rgba(255,255,255,0.1);
}
.settings-header h2 { font-size: 18px; }
.settings-header button {
  background: none; border: none;
  color: #aaa; font-size: 18px; cursor: pointer;
}
.settings-tabs {
  display: flex; gap: 0;
  border-bottom: 1px solid rgba(255,255,255,0.1);
}
.settings-tabs .tab {
  flex: 1; padding: 10px;
  background: none; border: none;
  color: #888; cursor: pointer;
  font-size: 14px;
  border-bottom: 2px solid transparent;
}
.settings-tabs .tab.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
}
.tab-content { padding: 20px; }
.tab-content.hidden { display: none; }
.form-group { margin-bottom: 14px; }
.form-group label { display: block; font-size: 12px; color: #888; margin-bottom: 4px; }
.form-group input, .form-group select, .form-group textarea {
  width: 100%; padding: 8px 12px;
  border-radius: 8px; border: 1px solid rgba(255,255,255,0.1);
  background: rgba(0,0,0,0.3); color: #cdd6f4;
  font-size: 13px;
}
.form-group textarea { resize: vertical; font-family: inherit; }
.form-row { display: flex; gap: 12px; }
.form-row .form-group { flex: 1; }
.btn-row { display: flex; gap: 8px; margin-top: 16px; }
.btn-row button {
  padding: 8px 20px; border-radius: 8px;
  border: 1px solid rgba(255,255,255,0.15);
  background: transparent; color: #cdd6f4;
  cursor: pointer; font-size: 13px;
}
.btn-row button.primary {
  background: var(--accent); color: #000; border: none;
}
#test-result { margin-top: 8px; font-size: 13px; }
#settings-panel { display: none; }
#settings-panel.open { display: block; }
```

- [ ] **Step 3: 提交**

```bash
git add frontend/
git commit -m "feat: add settings panel with config management UI"
```

---

### Task 9: 系统托盘 + Wails 配置

**Files:**
- Modify: `app.go` — 添加托盘逻辑
- Modify: `wails.json` — 配置窗口属性

- [ ] **Step 1: 更新 wails.json**

`wails.json`:
```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "trss",
  "outputfilename": "trss",
  "frontend:install": "",
  "frontend:build": "",
  "frontend:dev:watcher": "",
  "frontend:dev:serverUrl": "",
  "author": {
    "name": "xh1126xx",
    "email": "2373258819@qq.com"
  }
}
```

- [ ] **Step 2: 在 app.go 中添加系统托盘**

在 `app.go` 的 `Startup` 方法中添加托盘设置：
```go
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// 设置系统托盘
	runtime.WindowSetAlwaysOnTop(ctx, true)

	// 托盘菜单
	menu := runtime.NewMenu()
	menu.AddText("开始监听", false, func(_ *runtime.MenuItem) {
		// 由前端控制
	})
	menu.AddText("暂停/恢复", false, func(_ *runtime.MenuItem) {
		a.PauseListening()
	})
	menu.AddSeparator()
	menu.AddText("设置", false, func(_ *runtime.MenuItem) {
		runtime.WindowShow(ctx)
	})
	menu.AddText("退出", false, func(_ *runtime.MenuItem) {
		a.StopListening()
		runtime.Quit(ctx)
	})

	runtime.MenuSetApplicationMenu(ctx, menu)
}
```

- [ ] **Step 3: 提交**

```bash
git add app.go wails.json
git commit -m "feat: add system tray menu and window configuration"
```

---

### Task 10: 构建与最终测试

- [ ] **Step 1: 构建生产版本**

```bash
wails build
# 预期: 生成 build/bin/trss.exe (~12-18MB)
```

- [ ] **Step 2: 检查构建产物大小**

```bash
ls -lh build/bin/trss.exe
# 预期: 约 12-18MB
```

- [ ] **Step 3: 运行测试**

```bash
go test ./...
# 预期: PASS - all module tests pass
```

- [ ] **Step 4: 手动验证**

启动 `build/bin/trss.exe`，验证：
- 窗口正常显示
- 控制栏、字幕区、状态栏渲染正常
- 设置面板可打开/关闭
- 可创建/保存/切换配置方案
- 系统托盘图标和菜单正常

- [ ] **Step 5: 最终提交**

```bash
git add -A && git commit -m "feat: complete initial build

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"
```

---

### 后续优化（不包含在当前计划）

- 流式翻译（SSE）支持，进一步降低延迟
- 字幕窗口拖拽调整位置
- 开机自启动选项
- 快捷键控制（开始/暂停/切换模式）
- 音频缓冲区累积策略优化（避免 API 调用过于频繁）
