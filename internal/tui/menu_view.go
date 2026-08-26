package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/frandy/eudic-tui/internal/tui/theme"
)

// menuEntries 主菜单固定项
var menuEntries = []string{"听力练习", "背单词"}

// handleMenuKey 处理主菜单按键
func (m *appModel) handleMenuKey(msg tea.KeyMsg) []tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return []tea.Cmd{tea.Quit}
	case "j":
		if m.menu.cursor < len(menuEntries)-1 {
			m.menu.cursor++
		}
	case "k":
		if m.menu.cursor > 0 {
			m.menu.cursor--
		}
	case "enter":
		switch m.menu.cursor {
		case 0: // 听力练习
			m.state = stateList
			// 首次进入列表时触发加载，已有数据则直接展示
			if len(m.list.items) == 0 && !m.list.loading {
				m.list.loading = true
				m.list.err = ""
				return []tea.Cmd{loadListCmd(m.client, 0)}
			}
		case 1: // 背单词
			m.state = stateVocab
		}
	}
	return nil
}

// renderMenuView 渲染主菜单
func (m *appModel) renderMenuView() string {
	width := m.width
	if width < 40 {
		width = 60
	}

	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("🎧 Eudic TUI"))
	b.WriteString("\n\n")
	b.WriteString(theme.HelpStyle.Render("选择一个功能进入："))
	b.WriteString("\n\n")

	for i, entry := range menuEntries {
		cursor := "  "
		if i == m.menu.cursor {
			cursor = "▶ "
		}
		line := cursor + entry
		if i == m.menu.cursor {
			b.WriteString(theme.ActiveStyle.Render(line))
		} else {
			b.WriteString(theme.NormalStyle.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(theme.HelpStyle.Render("[j/k] 选择  [Enter] 进入  [q] 退出"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// handleVocabKey 处理背单词视图按键（仅占位）
func (m *appModel) handleVocabKey(msg tea.KeyMsg) []tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return []tea.Cmd{tea.Quit}
	case "esc", "backspace":
		m.state = stateMenu
	}
	return nil
}

// renderVocabView 渲染背单词占位视图
func (m *appModel) renderVocabView() string {
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("📖 背单词"))
	b.WriteString("\n\n")
	b.WriteString(theme.WarningStyle.Render("🚧 开发中，敬请期待 🚧"))
	b.WriteString("\n\n")
	b.WriteString(theme.HelpStyle.Render("[Esc] 返回主菜单  [q] 退出"))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}
