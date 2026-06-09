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
	Text    string `json:"text"`
	IsFinal bool   `json:"is_final"`
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

// Prompt 返回当前配置的原始提示词
func (c *Client) Prompt() string {
	return c.cfg.Prompt
}

// Config 返回当前配置
func (c *Client) Config() Config {
	return c.cfg
}

// buildSystemPrompt 用配置变量替换提示词中的占位符
func (c *Client) buildSystemPrompt() string {
	s := c.cfg.Prompt
	s = strings.ReplaceAll(s, "{source}", c.cfg.SourceLang)
	s = strings.ReplaceAll(s, "{target}", c.cfg.TargetLang)
	return s
}

// Translate 发送音频数据到多模态 API，返回翻译文本
func (c *Client) Translate(audio []byte) (*Result, error) {
	audioB64 := "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(audio)

	// 指令文本：放在和音频同一条消息中，确保模型执行翻译
	instruction := fmt.Sprintf(
		"请完成以下任务：\n1. 听这段%s音频，写出你听到的原文\n2. 将原文翻译为%s\n\n必须严格按格式输出（不要漏掉译文）：\n【原文】<原文内容>\n【译文】<%s翻译>",
		c.cfg.SourceLang, c.cfg.TargetLang, c.cfg.TargetLang,
	)

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
						"type": "text",
						"text": instruction,
					},
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

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
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
	return nil
}
