package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/frandy/eudic-tui/internal/models"
)

// Load 从指定路径加载 config.toml
// 若 configPath 为空，则按顺序查找：./config.toml, ~/.config/eudic-tui/config.toml
func Load(configPath string) (*models.AppConfig, error) {
	if configPath == "" {
		candidates := []string{
			"./config.toml",
			filepath.Join(os.Getenv("HOME"), ".config", "eudic-tui", "config.toml"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				configPath = c
				break
			}
		}
	}
	if configPath == "" {
		return nil, fmt.Errorf("未找到 config.toml，请拷贝 config.example.toml 为 config.toml 并填写 cookies")
	}

	cfg := &models.AppConfig{
		ChannelID:     "16287bc6-3ac0-4a9b-b06b-bc0224f2acb1",
		CacheDir:      "./cache",
		DefaultSpeed:  1.0,
		DefaultVolume: 80,
	}
	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, fmt.Errorf("解析 config.toml 失败: %w", err)
	}

	if cfg.EudicSession == "" {
		return nil, fmt.Errorf("config.toml 中 eudic_session 不能为空")
	}
	if cfg.ChannelID == "" {
		return nil, fmt.Errorf("config.toml 中 channel_id 不能为空")
	}
	return cfg, nil
}
