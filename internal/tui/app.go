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
	stateMenu appState = iota
	stateList
	statePlayer
	stateVocab
)

// menuModel 主菜单视图状态
type menuModel struct {
	cursor int
	// entries 固定两项：听力练习、背单词
}

// appModel 主模型
type appModel struct {
	state    appState
	client   *client.EudicClient
	cache    *cache.AudioCache
	progress *cache.ProgressStore
	cfg      *models.AppConfig

	menu        menuModel
	list        listModel
	player      playerModel
	lastSavedPos float64

	width, height int
}

// tickMsg 定时刷新（驱动进度条与句子高亮）
type tickMsg struct{}

// NewApp 创建主模型
func NewApp(cfg *models.AppConfig, c *client.EudicClient, ch *cache.AudioCache, ps *cache.ProgressStore) *appModel {
	return &appModel{
		state:    stateMenu,
		client:   c,
		cache:    ch,
		progress: ps,
		cfg:      cfg,
		menu:     menuModel{},
		list:     listModel{page: 0},
	}
}

// Init 初始化命令
func (m *appModel) Init() tea.Cmd {
	return nil
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
		case stateMenu:
			cmds = append(cmds, m.handleMenuKey(msg)...)
		case stateList:
			cmds = append(cmds, m.handleListKey(msg)...)
		case statePlayer:
			cmds = append(cmds, m.handlePlayerKey(msg)...)
		case stateVocab:
			cmds = append(cmds, m.handleVocabKey(msg)...)
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

// handleListKey 处理列表按键（vim 风格）
func (m *appModel) handleListKey(msg tea.KeyMsg) []tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return []tea.Cmd{tea.Quit}
	case "esc":
		m.state = stateMenu
		return nil
	case "r":
		m.list.loading = true
		m.list.err = ""
		m.list.page = 0
		m.list.items = nil
		return []tea.Cmd{loadListCmd(m.client, 0)}
	case "j":
		if m.list.cursor < len(m.list.items)-1 {
			m.list.cursor++
		}
		m.ensureListVisible()
	case "k":
		if m.list.cursor > 0 {
			m.list.cursor--
		}
		m.ensureListVisible()
	case "g":
		m.list.cursor = 0
		m.list.offset = 0
	case "G":
		if len(m.list.items) > 0 {
			m.list.cursor = len(m.list.items) - 1
		}
		m.ensureListVisible()
	case "ctrl+d":
		m.list.cursor += m.listHalfPage()
		if m.list.cursor >= len(m.list.items) {
			m.list.cursor = len(m.list.items) - 1
		}
		m.ensureListVisible()
	case "ctrl+u":
		m.list.cursor -= m.listHalfPage()
		if m.list.cursor < 0 {
			m.list.cursor = 0
		}
		m.ensureListVisible()
	case "enter":
		if len(m.list.items) == 0 {
			return nil
		}
		item := m.list.items[m.list.cursor]
		m.state = statePlayer
		m.player = playerModel{
			mediaID:    item.MediaID,
			mediaTitle: item.Title,
			loading:    true,
		}
		return []tea.Cmd{loadDetailCmd(m.client, m.cache, item)}
	}
	return nil
}

// listWinHeight 列表可视行数
func (m *appModel) listWinHeight() int {
	winH := m.height - 10
	if winH < 1 {
		winH = 14
	}
	return winH
}

// listHalfPage 翻半屏的行数（至少 1）
func (m *appModel) listHalfPage() int {
	h := m.listWinHeight() / 2
	if h < 1 {
		h = 1
	}
	return h
}

// ensureListVisible 让 cursor 保持在可视区域内
func (m *appModel) ensureListVisible() {
	if m.list.cursor < 0 {
		m.list.cursor = 0
	}
	if max := len(m.list.items) - 1; m.list.cursor > max && max >= 0 {
		m.list.cursor = max
	}
	winH := m.listWinHeight()
	if m.list.cursor < m.list.offset {
		m.list.offset = m.list.cursor
	} else if m.list.cursor >= m.list.offset+winH {
		m.list.offset = m.list.cursor - winH + 1
		if m.list.offset < 0 {
			m.list.offset = 0
		}
	}
}

// handlePlayerKey 处理播放器按键（vim 风格）
func (m *appModel) handlePlayerKey(msg tea.KeyMsg) []tea.Cmd {
	p := m.player.player
	if p == nil {
		// 音频还没就绪，只处理 Esc/q
		switch msg.String() {
		case "q", "ctrl+c":
			return []tea.Cmd{tea.Quit}
		case "esc", "backspace":
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
		m.player.player = nil
		return []tea.Cmd{tea.Quit}
	case "esc", "backspace":
		if m.progress != nil {
			_ = m.progress.Save(m.player.mediaID, p.Position())
		}
		p.Close()
		m.player.player = nil
		m.state = stateList
	case " ": // space
		p.Toggle()
	case "h":
		_ = p.Seek(p.Position() - 5)
	case "l":
		_ = p.Seek(p.Position() + 5)
	case "j":
		// 下一句
		if m.player.detail != nil && m.player.currentSentence < len(m.player.detail.Sentences)-1 {
			next := m.player.detail.Sentences[m.player.currentSentence+1]
			_ = p.Seek(next.Start)
		}
	case "k":
		// 上一句
		if m.player.detail != nil && m.player.currentSentence > 0 {
			prev := m.player.detail.Sentences[m.player.currentSentence-1]
			_ = p.Seek(prev.Start)
		}
	case "+", "=":
		p.SetVolume(p.Volume() + 0.05)
	case "-":
		p.SetVolume(p.Volume() - 0.05)
	case "[":
		p.SetSpeed(p.Speed() - 0.25)
	case "]":
		p.SetSpeed(p.Speed() + 0.25)
	case ".":
		if p.HasLoop() {
			p.ClearLoop()
		} else if m.player.detail != nil && m.player.currentSentence < len(m.player.detail.Sentences) {
			s := m.player.detail.Sentences[m.player.currentSentence]
			p.SetLoop(s.Start, s.End)
		}
	}
	return nil
}

// View 主渲染
func (m *appModel) View() string {
	switch m.state {
	case stateMenu:
		return m.renderMenuView()
	case statePlayer:
		return m.renderPlayerView()
	case stateVocab:
		return m.renderVocabView()
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
