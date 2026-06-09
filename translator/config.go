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
