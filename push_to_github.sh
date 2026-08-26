#!/usr/bin/env bash
# 一键推送到 GitHub：git@github.com:readmagic/eudic-tui.git
# 用法：在 /home/frandy/Downloads/eudic-tui 目录下执行 bash push_to_github.sh

set -e
cd "$(dirname "$0")"

REMOTE=git@github.com:readmagic/eudic-tui.git

# 确认不在错误目录
if [ ! -f go.mod ]; then
  echo "未找到 go.mod，请在项目根目录执行此脚本"
  exit 1
fi

# 初始化 git 仓库（如果还没初始化）
if [ ! -d .git ]; then
  git init -b main
fi

# 添加所有文件（.gitignore 会过滤掉 bin/ cache/ config.toml 等）
git add -A

# 确认暂存区没有 config.toml
if git diff --cached --name-only | grep -q '^config.toml$'; then
  echo "警告：config.toml 被暂存了，请检查 .gitignore"
  git reset config.toml 2>/dev/null || true
fi

# 检查是否有暂存文件
if ! git diff --cached --quiet; then
  git commit -m "$(cat <<'EOF'
初始化 Eudic 听力练习 TUI

基于 go-musicfox 技术栈（Go + bubbletea + beep/oto + bbolt + toml）
实现的欧路词典每日英语听力终端练习工具。

功能：
- 列表加载（自动重试应对偶发 EOF）
- 详情页解析（带时间戳的句子 + 中文翻译 + 题目图片占位）
- 音频下载（m4a → ffmpeg 转码 mp3 缓存）
- 播放控制（播放/暂停/跳转/变速/音量/单句循环）
- 当句高亮 + 自动滚动跟随
- 播放进度本地存储（bbolt）

作者：Frandy
EOF
)"
else
  echo "没有需要提交的变更"
fi

# 添加远程（如果还没添加）
if ! git remote get-url origin > /dev/null 2>&1; then
  git remote add origin "$REMOTE"
else
  git remote set-url origin "$REMOTE"
fi

# 推送
echo "推送到 $REMOTE ..."
git push -u origin main

echo "完成 ✓"
