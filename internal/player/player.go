package player

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/effects"
	"github.com/gopxl/beep/mp3"
	"github.com/gopxl/beep/speaker"
)

// Player 封装 beep 播放控制：播放/暂停、跳转、变速、音量、单句循环
type Player struct {
	mu       sync.Mutex
	src      beep.StreamSeekCloser
	format   beep.Format
	ctrl     *beep.Ctrl
	resamp   *beep.Resampler
	vol      *effects.Volume
	speed    float64
	volume   float64 // 0..1
	loop     *loopRange
	running  bool
	started  bool
}

type loopRange struct {
	start, end float64
}

// Open 打开本地 mp3 文件，初始化 Player（不启动播放）
func Open(path string) (*Player, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开音频文件失败: %w", err)
	}
	s, format, err := mp3.Decode(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("解码 mp3 失败: %w", err)
	}
	p := &Player{
		src:    s,
		format: format,
		speed:  1.0,
		volume: 1.0,
	}
	// 链路：src -> resampler(变速) -> volume(音量) -> ctrl(暂停)
	p.resamp = beep.ResampleRatio(3, 1.0, s)
	p.vol = &effects.Volume{Streamer: p.resamp, Base: 2, Volume: 0}
	p.ctrl = &beep.Ctrl{Streamer: p.vol}
	return p, nil
}

// InitSpeaker 初始化音频输出设备（必须先调用一次才能播放）
func (p *Player) InitSpeaker() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil
	}
	if err := speaker.Init(p.format.SampleRate, p.format.SampleRate.N(time.Second/10)); err != nil {
		return fmt.Errorf("初始化音频输出失败: %w", err)
	}
	p.running = true
	return nil
}

// Play 开始播放
func (p *Player) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	p.ctrl.Paused = false
	if !p.started {
		p.started = true
		speaker.Play(p.ctrl)
	}
}

// Pause 暂停
func (p *Player) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ctrl.Paused = true
}

// Toggle 切换播放/暂停
func (p *Player) Toggle() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ctrl.Paused = !p.ctrl.Paused
	if !p.started && !p.ctrl.Paused {
		p.started = true
		speaker.Play(p.ctrl)
	}
}

// IsPaused 当前是否暂停
func (p *Player) IsPaused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ctrl.Paused
}

// Seek 跳转到指定秒数
func (p *Player) Seek(sec float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	sample := int(sec * float64(p.format.SampleRate))
	if sample < 0 {
		sample = 0
	}
	if sample > p.src.Len() {
		sample = p.src.Len()
	}
	if !p.running {
		return p.src.Seek(sample)
	}
	// 锁住 speaker，避免 oto 驱动 goroutine 在 seek 期间并发读取解码器导致状态错乱。
	speaker.Lock()
	defer speaker.Unlock()
	if err := p.src.Seek(sample); err != nil {
		return err
	}
	// Resampler 无 Seek 方法，内部 buf/pos/off 仍停在旧位置，会输出旧样本甚至误判 EOF。
	// 保留当前 ratio 重建 resampler，并重连到 vol，让下一次 Stream 从新位置干净起步。
	p.resamp = beep.ResampleRatio(3, p.resamp.Ratio(), p.src)
	p.vol.Streamer = p.resamp
	return nil
}

// Position 返回当前播放位置（秒，音频时间）
func (p *Player) Position() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return float64(p.src.Position()) / float64(p.format.SampleRate)
}

// Duration 返回音频总时长（秒）
func (p *Player) Duration() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return float64(p.src.Len()) / float64(p.format.SampleRate)
}

// SetSpeed 设置播放速度（1.0=原速）
func (p *Player) SetSpeed(speed float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if speed < 0.25 {
		speed = 0.25
	}
	if speed > 4 {
		speed = 4
	}
	p.speed = speed
	speaker.Lock()
	defer speaker.Unlock()
	p.resamp.SetRatio(speed)
}

// Speed 当前速度
func (p *Player) Speed() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.speed
}

// SetVolume 设置音量（0..1）
func (p *Player) SetVolume(v float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.volume = v
	speaker.Lock()
	defer speaker.Unlock()
	if v == 0 {
		p.vol.Silent = true
	} else {
		p.vol.Silent = false
		// Base=2 时，Volume 范围 [-6, 0]，对应 1/64 ~ 1 倍增益
		p.vol.Volume = (v - 1) * 6
	}
}

// Volume 当前音量（0..1）
func (p *Player) Volume() float64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// SetLoop 设置单句循环区间，传 nil 取消
func (p *Player) SetLoop(start, end float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.loop = &loopRange{start: start, end: end}
}

// ClearLoop 取消单句循环
func (p *Player) ClearLoop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.loop = nil
}

// HasLoop 当前是否在循环
func (p *Player) HasLoop() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.loop != nil
}

// TickLoop 由外部定时调用（如 200ms），检查循环区间是否到 end
func (p *Player) TickLoop() {
	p.mu.Lock()
	loop := p.loop
	paused := p.ctrl.Paused
	p.mu.Unlock()
	if loop == nil || paused {
		return
	}
	pos := p.Position()
	if pos >= loop.end {
		_ = p.Seek(loop.start)
	}
}

// Close 释放资源
func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.src != nil {
		_ = p.src.Close()
	}
	if p.running {
		speaker.Clear()
		speaker.Close()
		p.running = false
	}
}
