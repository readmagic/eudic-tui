package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/frandy/eudic-tui/internal/models"
	"github.com/frandy/eudic-tui/internal/tui/theme"
)

// listModel 列表视图状态
type listModel struct {
	items   []models.Media
	cursor  int
	page    int
	loading bool
	err     string
	offset  int // 视图滚动偏移
}

// listLoadedMsg 列表加载完成
type listLoadedMsg struct {
	items []models.Media
	page  int
	err   error
}

// renderListView 渲染列表视图
func (m *appModel) renderListView() string {
	width := m.width
	if width < 80 {
		width = 100
	}
	height := m.height
	if height < 10 {
		height = 30
	}

	var b strings.Builder
	title := theme.TitleStyle.Render("🎧 听力列表")
	b.WriteString(title)
	b.WriteString("\n\n")

	if m.list.err != "" {
		b.WriteString(theme.ErrorStyle.Render("错误：" + m.list.err))
		b.WriteString("\n\n")
	}

	if m.list.loading && len(m.list.items) == 0 {
		b.WriteString(theme.HelpStyle.Render("正在加载列表..."))
		b.WriteString("\n")
	} else if len(m.list.items) == 0 {
		b.WriteString(theme.HelpStyle.Render("没有数据。按 r 刷新，按 q 退出。"))
		b.WriteString("\n")
	}

	// 表头：只保留序号和标题
	b.WriteString(theme.NormalStyle.Render(fmt.Sprintf("%-4s %s", "#", "标题")))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", width-2))
	b.WriteString("\n")

	// 列表项：cursor(2) + 序号(4) + 空格(1) + 标题(剩余)
	start := m.list.offset
	winHeight := height - 10
	if winHeight < 1 {
		winHeight = 1
	}
	end := start + winHeight
	if end > len(m.list.items) {
		end = len(m.list.items)
	}
	titleMaxWidth := width - 2 - 4 - 1
	if titleMaxWidth < 10 {
		titleMaxWidth = 10
	}
	for i := start; i < end; i++ {
		item := m.list.items[i]
		cursor := "  "
		if i == m.list.cursor {
			cursor = "▶ "
		}
		line := fmt.Sprintf("%s%-3d %s", cursor, i+1, truncate(item.Title, titleMaxWidth))
		if i == m.list.cursor {
			b.WriteString(theme.ActiveStyle.Render(line))
		} else {
			b.WriteString(theme.NormalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// 底部状态/帮助
	b.WriteString("\n")
	help := "[Enter] 播放  [j/k] 选择  [g/G] 顶/底  [Ctrl-d/u] 翻半屏  [r] 刷新  [Esc] 主菜单  [q] 退出"
	if len(m.list.items) > 0 {
		help = fmt.Sprintf("[%d/%d]  ", m.list.cursor+1, len(m.list.items)) + help
	}
	b.WriteString(theme.HelpStyle.Render(help))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// truncate 截断字符串到指定宽度（按 rune）
func truncate(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen-3]) + "..."
}
