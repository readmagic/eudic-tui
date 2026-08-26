package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/frandy/eudic-tui/internal/models"
	"github.com/frandy/eudic-tui/internal/player"
	"github.com/frandy/eudic-tui/internal/tui/theme"
)

// playerModel 播放视图状态
type playerModel struct {
	detail          *models.Detail
	player          *player.Player
	mediaID         string
	mediaTitle      string
	currentSentence int
	loading         bool
	err             string
	offset          int // 句子视图滚动偏移
}

// detailLoadedMsg 详情加载完成
type detailLoadedMsg struct {
	detail *models.Detail
	err    error
}

// audioReadyMsg 音频文件已就绪
type audioReadyMsg struct {
	path   string
	detail *models.Detail
	err    error
}

// progressMsg 播放进度更新
type progressMsg struct {
	pos      float64
	duration float64
	curSen   int
}

// renderPlayerView 渲染播放视图
func (m *appModel) renderPlayerView() string {
	width := m.width
	if width < 70 {
		width = 70
	}
	height := m.height
	if height < 20 {
		height = 40
	}

	var b strings.Builder
	p := m.player

	// 顶部信息行（不展示标题，只展示播放进度/速度/音量/状态）
	pos := 0.0
	dur := 0.0
	speed := 1.0
	vol := 1.0
	paused := true
	if p.player != nil {
		pos = p.player.Position()
		dur = p.player.Duration()
		speed = p.player.Speed()
		vol = p.player.Volume()
		paused = p.player.IsPaused()
	}
	header := fmt.Sprintf("%s / %s   %.2fx   %d%%",
		fmtTime(pos), fmtTime(dur), speed, int(vol*100))
	if paused {
		header += "  ⏸"
	} else {
		header += "  ▶"
	}
	b.WriteString(theme.TitleStyle.Render(header))
	b.WriteString("\n\n")

	if p.err != "" {
		b.WriteString(theme.ErrorStyle.Render("错误：" + p.err))
		b.WriteString("\n\n")
	}

	if p.loading {
		b.WriteString(theme.HelpStyle.Render("正在加载音频..."))
		b.WriteString("\n\n")
	} else if p.player == nil {
		b.WriteString(theme.HelpStyle.Render("音频未就绪"))
		b.WriteString("\n\n")
	}

	// 进度条
	barWidth := width - 4
	if barWidth < 10 {
		barWidth = 10
	}
	progress := 0.0
	if dur > 0 {
		progress = pos / dur
	}
	bar := renderProgressBar(progress, barWidth)
	b.WriteString(bar)
	b.WriteString("\n\n")

	// 句子列表
	if p.detail != nil && len(p.detail.Sentences) > 0 {
		sentences := p.detail.Sentences
		sentWinHeight := height - 14
		if sentWinHeight < 3 {
			sentWinHeight = 3
		}

		// 自动滚动跟随当前句
		cur := p.currentSentence
		if cur < p.offset {
			p.offset = cur
		}
		if cur >= p.offset+sentWinHeight {
			p.offset = cur - sentWinHeight + 1
		}
		end := p.offset + sentWinHeight
		if end > len(sentences) {
			end = len(sentences)
		}

		for i := p.offset; i < end; i++ {
			s := sentences[i]
			prefix := "  "
			style := theme.SentencePending
			if i < cur {
				style = theme.SentenceDone
				prefix = "✓ "
			} else if i == cur {
				style = theme.SentenceActive
				prefix = "▶ "
			}
			time := fmt.Sprintf("[%s] ", fmtTime(s.Start))
			line := fmt.Sprintf("%s%-10s %s", prefix, time, s.Text)
			b.WriteString(style.Render(truncate(line, width-4)))
			b.WriteString("\n")
		}
	}

	// 帮助
	b.WriteString("\n")
	help := "[Space] 播放/暂停  [h/l] ±5s  [j/k] 上下句  [+/-] 音量  [//] ±0.25x  [.] 单句循环  [Esc] 列表  [q] 退出"
	b.WriteString(theme.HelpStyle.Render(help))

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

// renderProgressBar 渲染进度条
func renderProgressBar(progress float64, width int) string {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	filled := int(progress * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("━", filled) + "●" + strings.Repeat("─", width-filled-1)
	if len(bar) > width+1 {
		bar = bar[:width+1]
	}
	return bar
}

// fmtTime 把秒数格式化为 M:SS
func fmtTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	m := int(sec) / 60
	s := int(sec) - m*60
	return fmt.Sprintf("%d:%02d", m, s)
}
