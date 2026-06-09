package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

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

	// 音频累积
	bufMu     sync.Mutex
	audioBuf  []byte
	sampleRate int
}

// NewApp 创建应用实例
func NewApp() *App {
	configDir, _ := os.UserConfigDir()
	configPath := filepath.Join(configDir, "trss", "configs.json")
	os.MkdirAll(filepath.Dir(configPath), 0755)

	return &App{
		configs:    translator.NewConfigStore(configPath),
		sampleRate: 16000,
	}
}

// startup 在 Wails 启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.WindowSetAlwaysOnTop(ctx, true)
}

// === 配置管理（暴露给前端）===

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

// GetFullConfig 返回完整配置（含 API Key），用于编辑
func (a *App) GetFullConfig(name string) (*translator.Config, error) {
	return a.configs.Load(name)
}

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

func (a *App) DeleteConfig(name string) error {
	return a.configs.Delete(name)
}

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

	if a.capture != nil && a.capture.IsRunning() {
		a.StopListening()
	}

	capture := audio.NewCapture(audio.DefaultConfig())
	a.translator = translator.NewClient(*cfg)
	a.capture = capture
	a.audioBuf = nil
	a.sampleRate = 16000

	// 音频帧回调：累积到缓冲区
	capture.SetFrameCallback(func(frame audio.Frame) {
		if !a.isListening {
			return
		}
		a.bufMu.Lock()
		a.audioBuf = append(a.audioBuf, frame.Data...)
		a.bufMu.Unlock()
	})

	// 定时发送：每 2 秒发送一次累积的音频
	go a.sendLoop()

	a.isListening = true

	if err := capture.Start(); err != nil {
		return fmt.Errorf("start capture: %w", err)
	}

	runtime.EventsEmit(a.ctx, "status", map[string]interface{}{
		"listening": true,
	})

	return nil
}

// sendLoop 定期发送累积的音频到翻译 API
func (a *App) sendLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !a.isListening {
				return
			}

			a.bufMu.Lock()
			if len(a.audioBuf) == 0 {
				a.bufMu.Unlock()
				continue
			}
			// 取出当前缓冲并清空
			pcmData := make([]byte, len(a.audioBuf))
			copy(pcmData, a.audioBuf)
			a.audioBuf = a.audioBuf[:0]
			a.bufMu.Unlock()

			// 包装为 WAV 格式
			wavData := pcmToWAV(pcmData, a.sampleRate, 1, 16)

			// 发送翻译请求
			result, err := a.translator.Translate(wavData)
			if err != nil {
				runtime.EventsEmit(a.ctx, "error", err.Error())
				continue
			}
			if result != nil && result.Text != "" {
				runtime.EventsEmit(a.ctx, "subtitle", result)
			}

		default:
			if !a.isListening {
				// 最后一次刷新剩余数据
				a.bufMu.Lock()
				remaining := len(a.audioBuf)
				a.bufMu.Unlock()
				if remaining == 0 {
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

// StopListening 停止监听并清空缓冲
func (a *App) StopListening() {
	a.isListening = false
	if a.capture != nil {
		a.capture.Stop()
	}
	a.bufMu.Lock()
	a.audioBuf = nil
	a.bufMu.Unlock()
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

// pcmToWAV 将原始 PCM 数据包装为 WAV 格式
func pcmToWAV(pcm []byte, sampleRate int, numChannels int, bitsPerSample int) []byte {
	var buf bytes.Buffer

	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := len(pcm)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))      // chunk size
	binary.Write(&buf, binary.LittleEndian, uint16(1))       // PCM format
	binary.Write(&buf, binary.LittleEndian, uint16(numChannels))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(&buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(&buf, binary.LittleEndian, uint16(bitsPerSample))

	// data chunk
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(pcm)

	return buf.Bytes()
}
