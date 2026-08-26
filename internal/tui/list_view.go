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
	title := theme.TitleStyle.Render("🎧 Eudic 听力练习")
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

	// 表头
	header := fmt.Sprintf("%-3s %-50s %-20s %-10s", "#", "标题", "时间", "状态")
	b.WriteString(theme.NormalStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", width-2))
	b.WriteString("\n")

	// 列表项
	start := m.list.offset
	winHeight := height - 10
	if winHeight < 1 {
		winHeight = 1
	}
	end := start + winHeight
	if end > len(m.list.items) {
		end = len(m.list.items)
	}
	for i := start; i < end; i++ {
		item := m.list.items[i]
		cursor := "  "
		if i == m.list.cursor {
			cursor = "▶ "
		}
		title := truncate(item.Title, width-50)
		line := fmt.Sprintf("%s%-2d %-50s %-20s %-10s",
			cursor, i+1, title, item.FileTime, item.Status)
		if i == m.list.cursor {
			b.WriteString(theme.ActiveStyle.Render(line))
		} else {
			b.WriteString(theme.NormalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// 底部状态/帮助
	b.WriteString("\n")
	help := "[Enter] 播放  [↑/↓] 选择  [r] 刷新  [q] 退出"
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
