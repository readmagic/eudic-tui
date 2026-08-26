package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
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
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "加载配置失败:", err)
		fmt.Fprintln(os.Stderr, "请拷贝 config.example.toml 为 config.toml 并填写 cookies")
		os.Exit(1)
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
