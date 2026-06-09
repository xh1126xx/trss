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

// App 应用主结构体
type App struct {
	ctx         context.Context
	capture     *audio.Capture
	translator  *translator.Client
	configs     *translator.ConfigStore
	isListening bool

	bufMu    sync.Mutex
	audioBuf []byte
	// 滑动窗口：保留一部分上一次的音频（1s 重叠）
	prevTail   []byte
}

func NewApp() *App {
	configDir, _ := os.UserConfigDir()
	configPath := filepath.Join(configDir, "trss", "configs.json")
	os.MkdirAll(filepath.Dir(configPath), 0755)
	return &App{configs: translator.NewConfigStore(configPath)}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	runtime.WindowSetAlwaysOnTop(ctx, true)
}

// === 配置管理 ===

func (a *App) GetConfigs() []translator.Config {
	names, _ := a.configs.List()
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

func (a *App) GetFullConfig(name string) (*translator.Config, error) {
	return a.configs.Load(name)
}

func (a *App) SaveConfig(name, baseURL, apiKey, model, sourceLang, targetLang, prompt string) error {
	return a.configs.Save(translator.Config{
		Name: name, BaseURL: baseURL, APIKey: apiKey, Model: model,
		SourceLang: sourceLang, TargetLang: targetLang, Prompt: prompt,
	})
}

func (a *App) DeleteConfig(name string) error { return a.configs.Delete(name) }

func (a *App) TestConnection(name string) error {
	cfg, err := a.configs.Load(name)
	if err != nil {
		return err
	}
	return translator.NewClient(*cfg).TestConnection()
}

// === 翻译控制 ===

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
	a.prevTail = nil

	capture.SetFrameCallback(func(frame audio.Frame) {
		if !a.isListening {
			return
		}
		a.bufMu.Lock()
		a.audioBuf = append(a.audioBuf, frame.Data...)
		a.bufMu.Unlock()
	})

	a.isListening = true
	go a.sendLoop()

	if err := capture.Start(); err != nil {
		return fmt.Errorf("start capture: %w", err)
	}

	runtime.EventsEmit(a.ctx, "status", map[string]interface{}{"listening": true})
	return nil
}

// overlapBytes 根据实际音频格式计算 1 秒重叠量
func (a *App) overlapBytes() int {
	sr, ch, bits := 48000, 2, 16
	if a.capture != nil {
		sr, ch, bits = a.capture.ActualFormat()
		if sr == 0 { sr = 48000 }
		if ch == 0 { ch = 2 }
		if bits == 0 { bits = 16 }
	}
	return sr * ch * bits / 8
}

func (a *App) sendLoop() {
	// 每 2 秒发送
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	cooldown := time.Time{}

	for range ticker.C {
		if !a.isListening {
			return
		}
		if time.Now().Before(cooldown) {
			continue
		}

		a.bufMu.Lock()
		bufLen := len(a.audioBuf)
		if bufLen < 8000 { // 至少 ~40ms 音频
			a.bufMu.Unlock()
			continue
		}

		// 取出数据 + 滑动窗口重叠
		pcmData := make([]byte, bufLen)
		copy(pcmData, a.audioBuf)
		a.audioBuf = a.audioBuf[:0]

		// 保留最后约 1 秒用于下一轮重叠
		overlap := a.overlapBytes()
		if len(a.prevTail) > 0 {
			// 在前方加上上一次的尾部
			pcmData = append(a.prevTail, pcmData...)
		}
		if len(pcmData) > overlap {
			a.prevTail = make([]byte, overlap)
			copy(a.prevTail, pcmData[len(pcmData)-overlap:])
		} else {
			a.prevTail = make([]byte, len(pcmData))
			copy(a.prevTail, pcmData)
		}
		a.bufMu.Unlock()

		// WAV 封装
		sr, ch, bits := a.capture.ActualFormat()
		if sr == 0 { sr = 48000 }
		if ch == 0 { ch = 2 }
		if bits == 0 { bits = 16 }
		wavData := pcmToWAV(pcmData, sr, ch, bits)

		// 发送到翻译 API
		result, err := a.translator.Translate(wavData)
		if err != nil {
			errStr := err.Error()
			runtime.EventsEmit(a.ctx, "error", errStr)
			if strings.Contains(errStr, "429") {
				cooldown = time.Now().Add(15 * time.Second)
			}
			continue
		}

		if result == nil || result.Text == "" || isPromptEcho(result.Text) {
			continue
		}

		// 解析原文+译文，如果失败则整段当译文
		sourceText, targetText := parseBilingual(result.Text)
		if targetText == "" {
			targetText = strings.TrimSpace(result.Text)
			sourceText = ""
		}
		if targetText == "" {
			continue
		}

		runtime.EventsEmit(a.ctx, "subtitle", map[string]interface{}{
			"source": sourceText,
			"target": targetText,
		})
	}
}

// parseBilingual 从 API 回复中提取原文和译文
func parseBilingual(text string) (source, target string) {
	text = strings.TrimSpace(text)

	// 尝试多种分隔方式
	patterns := []struct{ srcTag, tgtTag string }{
		{"【原文】", "【译文】"},
		{"原文:", "译文:"},
		{"原文：", "译文："},
		{"[原文]", "[译文]"},
		{"Source:", "Target:"},
		{"[Source]", "[Target]"},
	}

	for _, p := range patterns {
		srcIdx := strings.Index(text, p.srcTag)
		tgtIdx := strings.Index(text, p.tgtTag)
		if srcIdx >= 0 && tgtIdx > srcIdx {
			// 提取原文：从 srcTag 到 tgtTag 之间
			srcStart := srcIdx + len(p.srcTag)
			source = strings.TrimSpace(text[srcStart:tgtIdx])
			// 提取译文：从 tgtTag 之后
			tgtStart := tgtIdx + len(p.tgtTag)
			if tgtStart < len(text) {
				target = strings.TrimSpace(text[tgtStart:])
			}
			return
		}
	}

	// 尝试按 "\n\n" 分割：第一段原文，第二段译文
	parts := strings.SplitN(text, "\n\n", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}

	// 无法解析，整体当译文
	return "", text
}

func isPromptEcho(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 3 {
		return true
	}
	keywords := []string{"翻译", "translate", "字幕", "subtitle", "输出:", "保留原意", "你是", "you are", "assistant", "助手"}
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

func (a *App) StopListening() {
	a.isListening = false
	if a.capture != nil {
		a.capture.Stop()
	}
	a.bufMu.Lock()
	a.audioBuf = nil
	a.prevTail = nil
	a.bufMu.Unlock()
	runtime.EventsEmit(a.ctx, "status", map[string]interface{}{"listening": false})
}

func (a *App) PauseListening() {
	a.isListening = !a.isListening
	runtime.EventsEmit(a.ctx, "status", map[string]interface{}{"listening": a.isListening})
}

func pcmToWAV(pcm []byte, sampleRate, numChannels, bitsPerSample int) []byte {
	var buf bytes.Buffer
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := len(pcm)

	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(numChannels))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(&buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(&buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(dataSize))
	buf.Write(pcm)

	return buf.Bytes()
}
