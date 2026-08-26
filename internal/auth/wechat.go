// Package auth 实现微信扫码登录欧路词典的流程
//
// 流程概览：
//  1. FetchUUID: 访问 open.weixin.qq.com/connect/qrconnect 拿到二维码 UUID
//  2. RenderQRCodeForUUID: 本地生成二维码并渲染到终端
//  3. PollStatus: 长轮询 lp.open.weixin.qq.com/connect/l/qrconnect?uuid=... 监听扫码状态
//  4. ExchangeCodeForCookies: 用户确认后用 wx_code 跳转 dict.eudic.net，捕获 Set-Cookie
//
// 作者：Frandy
package auth

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	// appID 欧路词典嵌入的微信登录 appid（来自 dict.eudic.net 网页 WxLogin SDK）
	appID = "wxdd51e521a16e70d0"
	// redirectURI 扫码成功后微信携带 wx_code 跳转到这里
	redirectURI = "https://dict.eudic.net/account/oauthlogin/en/weixin"
	// qrConnectURL 二维码页面（含 UUID 提取）
	qrConnectURL = "https://open.weixin.qq.com/connect/qrconnect?appid=" + appID +
		"&scope=snsapi_login&redirect_uri=" + redirectURI + "&state=&self_redirect=default&style=white"
	// pollURLFmt 长轮询端点，~30s 超时
	pollURLFmt = "https://lp.open.weixin.qq.com/connect/l/qrconnect?uuid=%s&_=%d"
	// loginURLFmt 用 wx_code 换欧路词典认证 cookies
	loginURLFmt = "https://dict.eudic.net/account/oauthlogin/en/weixin?code=%s&state="
)

// LoginStatus 扫码状态
type LoginStatus int

const (
	StatusWaiting   LoginStatus = iota // 408 等待扫码
	StatusScanned                       // 404 已扫码待确认
	StatusConfirmed                     // 405 已确认
	StatusCanceled                      // 403 用户取消
	StatusExpired                       // 402 二维码过期
)

// PollResult 一次轮询的结果
type PollResult struct {
	Status LoginStatus
	WXCode string // 仅 StatusConfirmed 时有值
}

// WechatLogin 微信扫码登录会话
type WechatLogin struct {
	uuid   string
	client *http.Client
}

// NewWechatLogin 创建登录会话（独立 HTTP client，无 cookie）
func NewWechatLogin() *WechatLogin {
	return &WechatLogin{
		client: &http.Client{
			Timeout: 35 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig:   &tls.Config{},
				ForceAttemptHTTP2: false,
			},
		},
	}
}

// UUID 获取到的二维码 UUID（FetchUUID 后才有值）
func (w *WechatLogin) UUID() string { return w.uuid }

// FetchUUID 访问二维码页面并提取 UUID
//
// 页面返回的 HTML 里 UUID 有两种出现形式：
//   - 旧版：window.uuid = "xxx" 或 uuid:"xxx"（等号/冒号后带引号）
//   - 新版：var fordevtool = ".../qrconnect?uuid=xxx"（嵌在 URL 查询参数里，等号后无引号）
//
// 新版页面里还有 uuid="+G+(e..." 等 JS 拼接片段，用 {10,} 长度过滤掉
func (w *WechatLogin) FetchUUID() (string, error) {
	resp, err := w.client.Get(qrConnectURL)
	if err != nil {
		return "", fmt.Errorf("请求二维码页面失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("二维码页面返回状态码 %d", resp.StatusCode)
	}
	// 兼容新旧两种写法：引号变可选 + 长度 ≥10 防止误匹配 JS 里的短变量
	re := regexp.MustCompile(`uuid["']?\s*[:=]\s*["']?([A-Za-z0-9_-]{10,})["']?`)
	m := re.FindSubmatch(body)
	if len(m) < 2 {
		return "", fmt.Errorf("无法从二维码页面提取 UUID")
	}
	w.uuid = string(m[1])
	return w.uuid, nil
}

// PollStatus 长轮询扫码状态（约 25-30 秒一次返回）
//
// 返回的 JS 里包含 `window.wx_errcode=408;window.wx_code='';`
//
// 微信要求 Referer: https://open.weixin.qq.com/ 与浏览器 UA，否则扫码后可能返回非预期内容
func (w *WechatLogin) PollStatus() (PollResult, error) {
	if w.uuid == "" {
		return PollResult{}, fmt.Errorf("UUID 为空，请先 FetchUUID")
	}
	u := fmt.Sprintf(pollURLFmt, w.uuid, time.Now().UnixMilli())
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return PollResult{}, fmt.Errorf("构造轮询请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://open.weixin.qq.com/")
	resp, err := w.client.Do(req)
	if err != nil {
		return PollResult{}, fmt.Errorf("轮询失败: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	s := string(body)
	code := extractJSVar(s, "wx_errcode")
	switch code {
	case "408":
		return PollResult{Status: StatusWaiting}, nil
	case "404":
		return PollResult{Status: StatusScanned}, nil
	case "405":
		return PollResult{Status: StatusConfirmed, WXCode: extractJSVar(s, "wx_code")}, nil
	case "403":
		return PollResult{Status: StatusCanceled}, nil
	case "402":
		return PollResult{Status: StatusExpired}, nil
	case "":
		// 提取失败：打印 raw body 便于诊断，按等待重试
		fmt.Fprintf(os.Stderr, "  [debug] wx_errcode 提取失败，raw body (status=%d): %s\n", resp.StatusCode, s)
		return PollResult{Status: StatusWaiting}, nil
	default:
		return PollResult{}, fmt.Errorf("未知 wx_errcode=%s", code)
	}
}

// ExchangeCodeForCookies 用 wx_code 跳转欧路词典，捕获 Set-Cookie
//
// dict.eudic.net/account/oauthlogin/en/weixin?code={wx_code}&state= 服务器用 code 换
// access_token，建立会话后 302 跳转 returnUrl，并在 302 响应的 Set-Cookie 头里设置
// EudicWebSession 与 .AspNetCore.Session 两个 cookie
func (w *WechatLogin) ExchangeCodeForCookies(wxCode string) (eudicSession, aspNetSession string, err error) {
	if wxCode == "" {
		return "", "", fmt.Errorf("wx_code 为空")
	}
	// 不自动跟随重定向，便于捕获 302 的 Set-Cookie
	loginClient := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{},
			ForceAttemptHTTP2: false,
		},
	}
	u := fmt.Sprintf(loginURLFmt, wxCode)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", "", fmt.Errorf("构造登录请求失败: %w", err)
	}
	req.Header.Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36")
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	resp, err := loginClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("登录请求失败: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	// 服务器会 302 跳转，Set-Cookie 在 302 响应里
	for _, c := range resp.Cookies() {
		switch c.Name {
		case "EudicWebSession":
			eudicSession = c.Value
		case ".AspNetCore.Session":
			aspNetSession = c.Value
		}
	}
	if eudicSession == "" {
		return "", "", fmt.Errorf("未获取到 EudicWebSession cookie（状态码 %d）", resp.StatusCode)
	}
	return eudicSession, aspNetSession, nil
}

// extractJSVar 从 JS 片段里提取变量值
// 支持格式：`window.wx_errcode=408;`、`wx_errcode="408";`、`window.wx_code='abc';`
// 引号可选且单双引号都兼容（微信不同字段混用单双引号）
func extractJSVar(s, name string) string {
	re := regexp.MustCompile(regexp.QuoteMeta(name) + `\s*=\s*['"]?([A-Za-z0-9_-]+)['"]?`)
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
