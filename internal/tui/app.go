package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/frandy/eudic-tui/internal/cache"
	"github.com/frandy/eudic-tui/internal/client"
	"github.com/frandy/eudic-tui/internal/models"
	"github.com/frandy/eudic-tui/internal/player"
)

// appState 应用状态
type appState int

const (
	stateList appState = iota
	statePlayer
)

// appModel 主模型
type appModel struct {
	state    appState
	client   *client.EudicClient
	cache    *cache.AudioCache
	progress *cache.ProgressStore
	cfg      *models.AppConfig

	list       listModel
	player     playerModel
	lastSavedPos float64

	width, height int
	quit          bool
}

// tickMsg 定时刷新（驱动进度条与句子高亮）
type tickMsg struct{}

// NewApp 创建主模型
func NewApp(cfg *models.AppConfig, c *client.EudicClient, ch *cache.AudioCache, ps *cache.ProgressStore) *appModel {
	return &appModel{
		state:    stateList,
		client:   c,
		cache:    ch,
		progress: ps,
		cfg:      cfg,
		list:     listModel{page: 0},
		player:   playerModel{showTranslation: true},
	}
}

// Init 初始化命令
func (m *appModel) Init() tea.Cmd {
	return loadListCmd(m.client, m.list.page)
}

// loadListCmd 异步加载列表
func loadListCmd(c *client.EudicClient, page int) tea.Cmd {
	return func() tea.Msg {
		items, err := c.GetMediaList(page)
		return listLoadedMsg{items: items, page: page, err: err}
	}
}

// loadDetailCmd 异步加载详情并下载音频
func loadDetailCmd(c *client.EudicClient, ch *cache.AudioCache, m models.Media) tea.Cmd {
	return func() tea.Msg {
		d, err := c.GetDetail(m)
		if err != nil {
			return detailLoadedMsg{err: err}
		}
		m4aPath := ch.M4APath(d.MediaID)
		if !ch.ExistsM4A(d.MediaID) {
			if err := c.DownloadAudio(d, m4aPath); err != nil {
				return detailLoadedMsg{err: err}
			}
		}
		playPath, err := ch.EnsurePlayable(d.MediaID)
		if err != nil {
			return audioReadyMsg{err: err}
		}
		return audioReadyMsg{path: playPath, detail: d}
	}
}

// tickCmd 每 200ms 刷新一次
func tickCmd() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Update 主更新逻辑
func (m *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch m.state {
		case stateList:
			cmds = append(cmds, m.handleListKey(msg)...)
		case statePlayer:
			cmds = append(cmds, m.handlePlayerKey(msg)...)
		}
	case listLoadedMsg:
		m.list.loading = false
		if msg.err != nil {
			m.list.err = msg.err.Error()
		} else {
			m.list.err = ""
			if msg.page == 0 {
				m.list.items = msg.items
			} else {
				m.list.items = append(m.list.items, msg.items...)
			}
			if len(m.list.items) > 0 && m.list.cursor >= len(m.list.items) {
				m.list.cursor = len(m.list.items) - 1
			}
		}
	case detailLoadedMsg:
		if msg.err != nil {
			m.player.err = msg.err.Error()
			m.player.loading = false
		} else if msg.detail != nil {
			m.player.detail = msg.detail
		}
	case audioReadyMsg:
		m.player.loading = false
		if msg.err != nil {
			m.player.err = msg.err.Error()
			return m, nil
		}
		if msg.detail != nil {
			m.player.detail = msg.detail
		}
		p, err := player.Open(msg.path)
		if err != nil {
			m.player.err = err.Error()
			return m, nil
		}
		m.player.err = ""
		m.player.player = p
		if err := p.InitSpeaker(); err != nil {
			m.player.err = err.Error()
			return m, nil
		}
		p.SetVolume(float64(m.cfg.DefaultVolume) / 100.0)
		p.SetSpeed(m.cfg.DefaultSpeed)
		// 恢复上次进度
		if m.progress != nil {
			pos := m.progress.Load(m.player.mediaID)
			if pos > 0 {
				_ = p.Seek(pos)
			}
		}
		p.Play()
		cmds = append(cmds, tickCmd())
	case tickMsg:
		if m.state == statePlayer && m.player.player != nil {
			p := m.player.player
			pos := p.Position()
			dur := p.Duration()
			cur := findCurrentSentence(m.player.detail, pos)
			m.player.currentSentence = cur
			p.TickLoop()
			// 保存进度（每秒一次）
			if m.progress != nil && int(pos) != int(m.lastSavedPos) {
				_ = m.progress.Save(m.player.mediaID, pos)
				m.lastSavedPos = pos
			}
			_ = dur
			cmds = append(cmds, tickCmd())
		}
	}

	return m, tea.Batch(cmds...)
}

// lastSavedPos 用于进度保存去抖（字段在 appModel 中）

// handleListKey 处理列表按键
func (m *appModel) handleListKey(msg tea.KeyMsg) []tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quit = true
		return nil
	case "r":
		m.list.loading = true
		m.list.err = ""
		m.list.page = 0
		m.list.items = nil
		return []tea.Cmd{loadListCmd(m.client, 0)}
	case "up", "k":
		if m.list.cursor > 0 {
			m.list.cursor--
			if m.list.cursor < m.list.offset {
				m.list.offset = m.list.cursor
			}
		}
	case "down", "j":
		if m.list.cursor < len(m.list.items)-1 {
			m.list.cursor++
			winH := m.height - 10
			if winH < 1 {
				winH = 14
			}
			if m.list.cursor >= m.list.offset+winH {
				m.list.offset++
			}
		}
	case "enter":
		if len(m.list.items) == 0 {
			return nil
		}
		item := m.list.items[m.list.cursor]
		m.state = statePlayer
		m.player = playerModel{
			mediaID:         item.MediaID,
			mediaTitle:      item.Title,
			showTranslation: true,
			loading:         true,
		}
		return []tea.Cmd{loadDetailCmd(m.client, m.cache, item)}
	}
	return nil
}

// handlePlayerKey 处理播放器按键
func (m *appModel) handlePlayerKey(msg tea.KeyMsg) []tea.Cmd {
	p := m.player.player
	if p == nil {
		// 音频还没就绪，只处理 L/q/Esc
		switch msg.String() {
		case "q", "ctrl+c":
			m.quit = true
		case "l", "esc":
			m.state = stateList
		}
		return nil
	}
	switch msg.String() {
	case "q", "ctrl+c":
		if m.progress != nil {
			_ = m.progress.Save(m.player.mediaID, p.Position())
		}
		p.Close()
		m.quit = true
	case "l", "esc":
		if m.progress != nil {
			_ = m.progress.Save(m.player.mediaID, p.Position())
		}
		p.Close()
		m.player.player = nil
		m.state = stateList
	case " ": // space
		p.Toggle()
	case "left":
		_ = p.Seek(p.Position() - 5)
	case "right":
		_ = p.Seek(p.Position() + 5)
	case "[":
		p.SetSpeed(p.Speed() - 0.25)
	case "]":
		p.SetSpeed(p.Speed() + 0.25)
	case "up":
		p.SetVolume(p.Volume() + 0.05)
	case "down":
		p.SetVolume(p.Volume() - 0.05)
	case ".":
		if p.HasLoop() {
			p.ClearLoop()
		} else if m.player.detail != nil && m.player.currentSentence < len(m.player.detail.Sentences) {
			s := m.player.detail.Sentences[m.player.currentSentence]
			p.SetLoop(s.Start, s.End)
		}
	case "t":
		m.player.showTranslation = !m.player.showTranslation
	case "n":
		// 下一句
		if m.player.detail != nil && m.player.currentSentence < len(m.player.detail.Sentences)-1 {
			next := m.player.detail.Sentences[m.player.currentSentence+1]
			_ = p.Seek(next.Start)
		}
	case "p":
		// 上一句
		if m.player.detail != nil && m.player.currentSentence > 0 {
			prev := m.player.detail.Sentences[m.player.currentSentence-1]
			_ = p.Seek(prev.Start)
		}
	}
	return nil
}

// View 主渲染
func (m *appModel) View() string {
	switch m.state {
	case statePlayer:
		return m.renderPlayerView()
	default:
		return m.renderListView()
	}
}

// findCurrentSentence 根据时间找到当前句子
func findCurrentSentence(d *models.Detail, pos float64) int {
	if d == nil || len(d.Sentences) == 0 {
		return 0
	}
	for i, s := range d.Sentences {
		if pos < s.End {
			if pos < s.Start {
				if i > 0 {
					return i - 1
				}
			}
			return i
		}
	}
	return len(d.Sentences) - 1
}

// lastSavedPos 字段（嵌入到 appModel，因为 Go 没有 _lastPosMarker 用法）
// 修正：放到 appModel 里直接定义
