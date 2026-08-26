// Package auth 中的 qrterm.go：把二维码以 Unicode 半字符 (▀▄█) 渲染到终端
// 高度压缩为原来的一半，每个终端字符行表示两个二维码模块
//
// 作者：Frandy
package auth

import (
	"fmt"
	"strings"

	goqrcode "github.com/skip2/go-qrcode"
)

// RenderQRCodeForUUID 为给定 UUID 生成二维码并以 Unicode 块字符渲染
//
// 扫码内容为 `https://open.weixin.qq.com/connect/qrcode/{uuid}`，
// 微信 App 扫码后会向微信服务器确认，从而触发后续轮询成功事件
func RenderQRCodeForUUID(uuid string) (string, error) {
	if uuid == "" {
		return "", fmt.Errorf("uuid 为空")
	}
	content := "https://open.weixin.qq.com/connect/qrcode/" + uuid
	q, err := goqrcode.New(content, goqrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("生成二维码失败: %w", err)
	}
	bitmap := q.Bitmap()
	// 加 4 模块 quiet zone（QR 标准要求）
	pad := 4
	size := len(bitmap) + pad*2
	grid := make([][]bool, size)
	for i := range grid {
		grid[i] = make([]bool, size)
	}
	for y, row := range bitmap {
		for x, v := range row {
			grid[y+pad][x+pad] = v
		}
	}
	// 用半字符渲染：每两个垂直模块合并为一个字符
	// true=true → █，仅上=▀，仅下=▄，都 false=空格
	var b strings.Builder
	for y := 0; y < size; y += 2 {
		// 左侧留白让二维码居中
		b.WriteString(strings.Repeat(" ", 2))
		for x := 0; x < size; x++ {
			top := grid[y][x]
			var bottom bool
			if y+1 < size {
				bottom = grid[y+1][x]
			}
			switch {
			case top && bottom:
				b.WriteString("█")
			case top && !bottom:
				b.WriteString("▀")
			case !top && bottom:
				b.WriteString("▄")
			default:
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}
