package translator

import (
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

	store.Save(Config{Name: "Profile A", BaseURL: "https://a.com", APIKey: "k1", Model: "m1"})
	store.Save(Config{Name: "Profile B", BaseURL: "https://b.com", APIKey: "k2", Model: "m2"})

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

func TestConfigStoreHandlesEmptyFile(t *testing.T) {
	// 确保空目录下 List 不出错
	tmpDir := t.TempDir()
	store := NewConfigStore(filepath.Join(tmpDir, "nonexistent.json"))

	profiles, err := store.List()
	if err != nil {
		t.Fatalf("List on nonexistent file should not error: %v", err)
	}
	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}
}
