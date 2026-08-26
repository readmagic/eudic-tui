# Eudic 听力 TUI

基于 [go-musicfox](https://github.com/go-musicfox/go-musicfox) 技术栈实现的欧路词典每日英语听力 TUI 练习工具。

## 功能

- 列出已上传的听力材料（自动分页）
- 播放音频（m4a 自动转码 mp3 缓存）
- 跟随当前播放位置高亮句子
- 显示中文翻译（按 `t` 切换）
- 速度调节（`[`/`]` ±0.25x）
- 单句循环（`.`）
- 音量调节（`↑`/`↓`）
- 快进/快退（`←`/`→` ±5s）
- 播放进度本地存储（bbolt）

## 安装

```bash
# 拷贝配置
cp config.example.toml config.toml
# 编辑 config.toml 填入欧路词典的 cookies
$EDITOR config.toml

# 构建
go build -o bin/eudic-tui ./cmd/eudic-tui

# 运行
./bin/eudic-tui
```

## 配置说明

打开 https://my.eudic.net/ting/index 登录后，按 F12 → Application → Cookies → 复制：

- `EudicWebSession` 的值 → 填 `eudic_session`
- `.AspNetCore.Session` 的值 → 填 `aspnet_session`

## 快捷键

列表界面：
- `↑`/`↓` 选择
- `Enter` 进入播放
- `r` 刷新
- `q` 退出

播放界面：
- `Space` 播放/暂停
- `←`/`→` ±5s
- `[`/`]` ±0.25x
- `↑`/`↓` 音量
- `.` 单句循环切换
- `t` 切换译文
- `L` 返回列表
- `q` 退出

## 技术栈

- TUI: `charmbracelet/bubbletea` + `bubbles` + `lipgloss`
- HTTP: `imroc/req/v3`
- 配置: `knadh/koanf` + `BurntSushi/toml`
- 存储: `go.etcd.io/bbolt`
- 音频: `gopxl/beep` + `ebitengine/oto/v3`
- 转码: 系统 `ffmpeg`
- HTML: `PuerkitoBio/goquery`

作者：Frandy
