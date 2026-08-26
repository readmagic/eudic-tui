// Package auth 中的 browser.go：用本地浏览器登录欧路词典并捕获 cookies
//
// 思路：启动一个可见的 Edge（Chromium 内核），导航到 dict.eudic.net 登录页，
// 用户在浏览器里完成登录（账号密码或微信扫码均可），程序每 2 秒查一次 cookies，
// 拿到 EudicWebSession 即视为登录成功，关闭浏览器并返回 cookies
//
// 作者：Frandy
package auth

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// RunLogin 启动 Edge 让用户登录欧路词典，捕获 cookies
//
// 超时 5 分钟，期间用户在浏览器里自由登录即可
func RunLogin() (eudicSession, aspNetSession string, err error) {
	// 查找 Edge 路径（Chromium 内核，chromedp 能驱动）
	edgePaths := []string{
		"/usr/bin/microsoft-edge",
		"/usr/bin/microsoft-edge-stable",
		"/opt/microsoft/msedge/msedge",
	}
	var execPath string
	for _, p := range edgePaths {
		if _, statErr := os.Stat(p); statErr == nil {
			execPath = p
			break
		}
	}
	if execPath == "" {
		return "", "", fmt.Errorf("未找到 Edge 浏览器，请安装 Microsoft Edge 或修改 browser.go")
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(execPath),
		chromedp.Flag("headless", false),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36 Edg/151.0.0.0"),
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	fmt.Fprintln(os.Stderr, "  已打开 Edge 浏览器，请在里面登录欧路词典")
	fmt.Fprintln(os.Stderr, "  支持账号密码 或 微信扫码登录")
	fmt.Fprintln(os.Stderr, "  登录成功后程序会自动捕获 cookies 并关闭浏览器")
	fmt.Fprintln(os.Stderr, "  超时时间 5 分钟，请尽快完成登录")

	err = chromedp.Run(ctx,
		chromedp.Navigate("https://dict.eudic.net/account/login"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
				}
				cookies, cerr := network.GetCookies().Do(ctx)
				if cerr != nil {
					continue
				}
				for _, c := range cookies {
					switch c.Name {
					case "EudicWebSession":
						eudicSession = c.Value
					case ".AspNetCore.Session":
						aspNetSession = c.Value
					}
				}
				if eudicSession != "" {
					fmt.Fprintln(os.Stderr, "  登录成功 ✓")
					return nil
				}
			}
		}),
	)
	if err != nil {
		return "", "", fmt.Errorf("浏览器登录失败: %w", err)
	}
	return eudicSession, aspNetSession, nil
}
