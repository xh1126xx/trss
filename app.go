package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// 每 5 秒发送一次，避免 429 限流
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	cooldown := time.Time{} // 429 退避冷却
	minAudioBytes := a.sampleRate * 2 / 2 // 至少 0.5 秒音频 (~8KB)

	for {
		select {
		case <-ticker.C:
			if !a.isListening {
				return
			}

			// 如果在冷却期，跳过
			if time.Now().Before(cooldown) {
				continue
			}

			a.bufMu.Lock()
			bufLen := len(a.audioBuf)
			if bufLen < minAudioBytes {
				a.bufMu.Unlock()
				continue
			}
			pcmData := make([]byte, bufLen)
			copy(pcmData, a.audioBuf)
			a.audioBuf = a.audioBuf[:0]
			a.bufMu.Unlock()

			// 使用系统实际音频格式封装为 WAV
			sr, ch, bits := a.capture.ActualFormat()
			if sr == 0 {
				sr = 48000
			}
			if ch == 0 {
				ch = 2
			}
			if bits == 0 {
				bits = 16
			}
			wavData := pcmToWAV(pcmData, sr, ch, bits)

			// 发送翻译请求
			result, err := a.translator.Translate(wavData)
			if err != nil {
				errStr := err.Error()
				runtime.EventsEmit(a.ctx, "error", errStr)
				// 429 限流 → 冷却 15 秒
				if strings.Contains(errStr, "429") {
					cooldown = time.Now().Add(15 * time.Second)
				}
				continue
			}
			if result != nil && result.Text != "" {
				// 过滤掉只返回了提示词本身的无效结果
				if !isPromptEcho(result.Text, a.translator.Prompt()) {
					runtime.EventsEmit(a.ctx, "subtitle", result)
				}
			}

		default:
			if !a.isListening {
				return
			}
		}
	}
}

// isPromptEcho 检查返回文本是否是无效回显（提示词、指令等非翻译内容）
func isPromptEcho(text, prompt string) bool {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return true
	}
	// 太短的不是正常翻译
	if len(text) < 5 {
		return true
	}
	// 包含翻译指令关键词的回显
	keywords := []string{"翻译", "translate", "字幕", "subtitle", "输出", "简洁", "保留原意"}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	// 纯语言代码（如 "en"、"zh"、"chinese"）
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= 15 && (strings.Contains(trimmed, "为") || strings.Contains(trimmed, "to")) {
		return true
	}
	return false
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
