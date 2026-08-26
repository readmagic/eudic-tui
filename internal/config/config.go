// Package config 加载与保存 eudic-tui 的 config.toml
//
// 配置查找顺序：./config.toml → ~/.config/eudic-tui/config.toml
// 若不存在或 cookies 缺失，仍返回默认 cfg 与 needsLogin=true，
// 由调用方决定是否启动微信扫码登录补齐 cookies。
//
// 作者：Frandy
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/frandy/eudic-tui/internal/models"
)

// defaultConfigPath 配置文件默认路径
func defaultConfigPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "eudic-tui", "config.toml")
}

// Load 加载 config.toml
//
// 行为：
//   - 找不到 config 文件：返回默认 cfg（needsLogin=true），由 main 启动登录
//   - 找到但 cookies 缺失：返回 cfg（needsLogin=true），由 main 启动登录
//   - 找到且 cookies 完整：返回 cfg（needsLogin=false）
func Load(configPath string) (cfg *models.AppConfig, needsLogin bool, err error) {
	if configPath == "" {
		candidates := []string{
			"./config.toml",
			defaultConfigPath(),
		}
		for _, c := range candidates {
			if _, statErr := os.Stat(c); statErr == nil {
				configPath = c
				break
			}
		}
	}

	c := &models.AppConfig{
		ChannelID:     "16287bc6-3ac0-4a9b-b06b-bc0224f2acb1",
		CacheDir:      "./cache",
		DefaultSpeed:  1.0,
		DefaultVolume: 80,
	}
	c.ConfigPath = configPath

	if configPath == "" {
		// 没找到配置文件，使用默认值并触发登录
		return c, true, nil
	}
	if _, decErr := toml.DecodeFile(configPath, c); decErr != nil {
		return nil, false, fmt.Errorf("解析 config.toml 失败: %w", decErr)
	}
	if c.CacheDir == "" {
		c.CacheDir = "./cache"
	}
	if c.ChannelID == "" {
		c.ChannelID = "16287bc6-3ac0-4a9b-b06b-bc0224f2acb1"
	}
	if c.EudicSession == "" {
		return c, true, nil
	}
	return c, false, nil
}

// Save 把 cfg 写回 config.toml（含 ConfigPath 指定的路径）
func Save(cfg *models.AppConfig) error {
	path := cfg.ConfigPath
	if path == "" {
		path = defaultConfigPath()
		cfg.ConfigPath = path
	}
	if dir := filepath.Dir(path); dir != "" {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return fmt.Errorf("创建配置目录失败: %w", mkErr)
		}
	}
	buf, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化 config 失败: %w", err)
	}
	if wErr := os.WriteFile(path, buf, 0o600); wErr != nil {
		return fmt.Errorf("写入 config 失败: %w", wErr)
	}
	return nil
}
