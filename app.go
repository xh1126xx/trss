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
	ctx         context.Context
	capture     *audio.Capture
	translator  *translator.Client
	configs     *translator.ConfigStore
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

// startup 在 Wails 启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 始终置顶
	runtime.WindowSetAlwaysOnTop(ctx, true)
}

// === 配置管理（暴露给前端）===

// GetConfigs 返回所有配置方案列表（隐藏 API Key）
func (a *App) GetConfigs() []translator.Config {
	names, err := a.configs.List()
	if err != nil {
		return nil
	}
	var configs []translator.Config
	for _, name := range names {
		cfg, err := a.configs.Load(name)
		if err != nil {
			continue
		}
		// 不暴露真实 API Key 给前端，仅返回掩码版本
		masked := *cfg
		if len(masked.APIKey) > 4 {
			masked.APIKey = masked.APIKey[:4] + "••••••"
		} else {
			masked.APIKey = "••••••"
		}
		configs = append(configs, masked)
	}
	return configs
}

// SaveConfig 保存配置方案（前端 JSON 字段用 snake_case）
func (a *App) SaveConfig(name, baseURL, apiKey, model, sourceLang, targetLang, prompt string) error {
	cfg := translator.Config{
		Name:       name,
		BaseURL:    baseURL,
		APIKey:     apiKey,
		Model:      model,
		SourceLang: sourceLang,
		TargetLang: targetLang,
		Prompt:     prompt,
	}
	return a.configs.Save(cfg)
}

// DeleteConfig 删除配置方案
func (a *App) DeleteConfig(name string) error {
	return a.configs.Delete(name)
}

// TestConnection 测试指定配置的 API 连通性
func (a *App) TestConnection(name string) error {
	cfg, err := a.configs.Load(name)
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

	// 如果已经在运行，先停止
	if a.capture != nil && a.capture.IsRunning() {
		a.StopListening()
	}

	// 初始化音频捕获
	capture := audio.NewCapture(audio.DefaultConfig())

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

	if err := capture.Start(); err != nil {
		return fmt.Errorf("start capture: %w", err)
	}

	runtime.EventsEmit(a.ctx, "status", map[string]interface{}{
		"listening": true,
	})

	return nil
}

// StopListening 停止监听
func (a *App) StopListening() {
	a.isListening = false
	if a.capture != nil {
		a.capture.Stop()
	}
	runtime.EventsEmit(a.ctx, "status", map[string]interface{}{
		"listening": false,
	})
}

// PauseListening 暂停/恢复监听
func (a *App) PauseListening() {
	a.isListening = !a.isListening
	runtime.EventsEmit(a.ctx, "status", map[string]interface{}{
		"listening": a.isListening,
	})
}
