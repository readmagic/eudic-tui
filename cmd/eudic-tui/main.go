// Package main eudic-tui 入口
//
// 启动流程：
//  1. 加载 config.toml，若不存在或 cookies 缺失 → 启动微信扫码登录补齐
//  2. 创建 client/cache/progress
//  3. 启动 bubbletea TUI
//
// 作者：Frandy
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/frandy/eudic-tui/internal/auth"
	"github.com/frandy/eudic-tui/internal/cache"
	"github.com/frandy/eudic-tui/internal/client"
	"github.com/frandy/eudic-tui/internal/config"
	"github.com/frandy/eudic-tui/internal/tui"
)

func main() {
	configPath := ""
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	cfg, needsLogin, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "加载配置失败:", err)
		os.Exit(1)
	}

	if needsLogin {
		fmt.Fprintln(os.Stderr, "未检测到有效的 cookies，启动微信扫码登录...")
		eudicSession, aspNetSession, lerr := auth.RunLogin()
		if lerr != nil {
			fmt.Fprintln(os.Stderr, "微信登录失败:", lerr)
			fmt.Fprintln(os.Stderr, "也可手动拷贝 config.example.toml 为 config.toml 并填写 cookies")
			os.Exit(1)
		}
		cfg.EudicSession = eudicSession
		cfg.AspNetSession = aspNetSession
		if serr := config.Save(cfg); serr != nil {
			fmt.Fprintln(os.Stderr, "保存 config.toml 失败:", serr)
		} else {
			fmt.Fprintln(os.Stderr, "cookies 已保存到", cfg.ConfigPath)
		}
	}

	c := client.NewClient(cfg)
	ch, err := cache.NewAudioCache(cfg.CacheDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化缓存目录失败:", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(cfg.CacheDir, "progress.db")
	ps, err := cache.NewProgressStore(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化进度数据库失败:", err)
		os.Exit(1)
	}
	defer ps.Close()

	app := tui.NewApp(cfg, c, ch, ps)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "运行 TUI 失败:", err)
		os.Exit(1)
	}
}
