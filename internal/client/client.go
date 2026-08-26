package client

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/frandy/eudic-tui/internal/models"
	"github.com/frandy/eudic-tui/internal/parser"
)

// EudicClient 封装欧路词典听力的三个 API 调用
type EudicClient struct {
	http    *http.Client
	headers http.Header
	cfg     *models.AppConfig
}

// NewClient 创建一个装载了 cookies 和公共 headers 的 HTTP 客户端
func NewClient(cfg *models.AppConfig) *EudicClient {
	cookieParts := []string{
		"EudicWebSession=" + cfg.EudicSession,
		"EudicUserLastActiveDate=2026-09-10",
		"col_index=6",
		"col_sort=desc",
	}
	if cfg.AspNetSession != "" {
		cookieParts = append(cookieParts, ".AspNetCore.Session="+cfg.AspNetSession)
	}
	h := http.Header{}
	h.Set("accept", "text/html, */*; q=0.01")
	h.Set("accept-language", "zh-CN,zh;q=0.9,en;q=0.8")
	h.Set("x-requested-with", "XMLHttpRequest")
	h.Set("referer", "https://my.eudic.net/ting/index")
	h.Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36 Edg/151.0.0.0")
	h.Set("sec-fetch-dest", "empty")
	h.Set("sec-fetch-mode", "cors")
	h.Set("sec-fetch-site", "same-origin")
	h.Set("cookie", strings.Join(cookieParts, "; "))
	c := &EudicClient{
		http: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error { return nil },
			// 欧路词典在 HTTP/2 下对长 cookie 会卡住，强制 HTTP/1.1
			Transport: &http.Transport{
				TLSClientConfig:    &tls.Config{},
				ForceAttemptHTTP2: false,
				DisableKeepAlives:  false,
			},
		},
		headers: h,
		cfg:     cfg,
	}
	return c
}

// do 发起 GET 请求并返回 body 字节（带 3 次重试以应对偶发 EOF）
func (c *EudicClient) do(rawURL string) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, 0, err
		}
		req.Header = c.headers.Clone()
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			continue
		}
		return body, resp.StatusCode, nil
	}
	return nil, 0, lastErr
}

// GetMediaList 调用列表 API，返回听力材料列表
func (c *EudicClient) GetMediaList(page int) ([]models.Media, error) {
	u := fmt.Sprintf("https://my.eudic.net/ting/GetPrivateMedias?sortby=0&page=%d&channelId=%s", page, c.cfg.ChannelID)
	body, status, err := c.do(u)
	if err != nil {
		return nil, fmt.Errorf("请求列表失败: %w", err)
	}
	if status != 200 {
		return nil, fmt.Errorf("列表 API 返回状态码 %d", status)
	}
	return parser.ParseMediaList(body)
}

// GetDetail 调用详情页 API，返回解析后的详情
func (c *EudicClient) GetDetail(m models.Media) (*models.Detail, error) {
	// token 含空格，需 URL 编码为 %20
	token := url.QueryEscape(m.DetailToken)
	token = strings.ReplaceAll(token, "+", "%20")
	u := fmt.Sprintf("https://dict.eudic.net/webting/desktopplay?id=%s&token=%s", m.MediaID, token)
	body, status, err := c.do(u)
	if err != nil {
		return nil, fmt.Errorf("请求详情页失败: %w", err)
	}
	if status != 200 {
		return nil, fmt.Errorf("详情 API 返回状态码 %d", status)
	}
	d, err := parser.ParseDetail(body)
	if err != nil {
		return nil, err
	}
	if d.MediaID == "" {
		d.MediaID = m.MediaID
	}
	if d.Title == "" {
		d.Title = m.Title
	}
	return d, nil
}

// DownloadAudio 下载音频文件到指定路径
// 音频 URL 是 public 的（token 已含鉴权），不需要 cookie，避免 referer 干扰
func (c *EudicClient) DownloadAudio(d *models.Detail, destPath string) error {
	if d.AudioURL == "" {
		return fmt.Errorf("音频 URL 为空")
	}
	audioURL := strings.ReplaceAll(d.AudioURL, " ", "%20")
	dlClient := &http.Client{
		Timeout:       60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return nil },
		Transport: &http.Transport{
			TLSClientConfig:    &tls.Config{},
			ForceAttemptHTTP2: false,
		},
	}
	req, err := http.NewRequest(http.MethodGet, audioURL, nil)
	if err != nil {
		return fmt.Errorf("构造音频请求失败: %w", err)
	}
	req.Header.Set("user-agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36")
	resp, err := dlClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求音频失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("音频 API 返回状态码 %d", resp.StatusCode)
	}
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}
