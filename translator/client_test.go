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
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("missing or wrong auth header")
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("wrong content-type: %s", ct)
		}
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
