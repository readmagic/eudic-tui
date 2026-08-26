package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/frandy/eudic-tui/internal/models"
)

// ParseMediaList 解析列表 API 返回的 HTML，提取听力材料列表
func ParseMediaList(html []byte) ([]models.Media, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}
	var list []models.Media
	doc.Find(".row.list-group-item").Each(func(_ int, s *goquery.Selection) {
		mediaID := s.Find("input.media_id").AttrOr("data", "")
		if mediaID == "" {
			return
		}
		title := s.Find("a.rl-title").AttrOr("title", "")
		fileTime := strings.TrimSpace(s.Find("span.file_time").Text())
		status := strings.TrimSpace(s.Find("span.badge.taskFinish").Text())
		href := s.Find(`a[href*="desktopplay"]`).AttrOr("href", "")
		detailToken := extractToken(href)
		list = append(list, models.Media{
			MediaID:     mediaID,
			Title:       title,
			FileTime:    fileTime,
			Status:      status,
			DetailToken: detailToken,
		})
	})
	return list, nil
}

// extractToken 从 desktopplay 的 URL 中提取 token 参数值
// href 形如 https://dict.eudic.net/webting/desktopplay?id=xxx&token=QYN%2BeyJ...
func extractToken(href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return u.Query().Get("token")
}

// translateObj 对应详情页里的 var translate = {...}
type translateObj struct {
	Translation []struct {
		Order     int    `json:"order"`
		Timestamp string `json:"timestamp"`
		Text      string `json:"text"`
	} `json:"translation"`
}

// ParseDetail 解析详情页 HTML，提取标题、句子、音频 URL
func ParseDetail(html []byte) (*models.Detail, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 失败: %w", err)
	}

	mediaID, audioURL := extractMediaIDAndAudio(html)
	detail := &models.Detail{
		MediaID:  mediaID,
		Title:    strings.TrimSpace(doc.Find("h1:not(.logo)").First().Text()),
		AudioURL: audioURL,
	}

	// 解析句子
	idx := 0
	doc.Find(".article .sentence").Each(func(_ int, s *goquery.Selection) {
		id, _ := s.Attr("id")
		// 跳过 h1 标题内嵌的 sentence（无 id 或不以 J_ 开头）
		if !strings.HasPrefix(id, "J_") {
			return
		}
		startStr, _ := s.Attr("data-starttime")
		endStr, _ := s.Attr("data-endtime")
		start := parseStamp(startStr)
		end := parseStamp(endStr)
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		detail.Sentences = append(detail.Sentences, models.Sentence{
			Index: idx,
			ID:    id,
			Start: start,
			End:   end,
			Text:  text,
		})
		idx++
	})

	// 解析 translate JSON 并按 timestamp 配对到 sentence.start
	trans := parseTranslateJSON(html)
	if len(trans) > 0 {
		stampIdx := map[string]int{}
		for i, s := range detail.Sentences {
			stampIdx[startStamp(s.Start)] = i
		}
		for _, t := range trans {
			if strings.Contains(t.Text, "<img") {
				continue
			}
			if i, ok := stampIdx[t.Timestamp]; ok {
				detail.Sentences[i].Translation = appendTrans(detail.Sentences[i].Translation, t.Text)
			}
		}
	}

	return detail, nil
}

// appendTrans 拼接同一句的多条翻译（去重空字符串）
func appendTrans(old, nw string) string {
	if nw == "" {
		return old
	}
	if old == "" {
		return nw
	}
	if strings.Contains(old, nw) {
		return old
	}
	return old + " " + nw
}

// parseTranslateJSON 从 HTML 中提取 var translate = {...} 并解析
func parseTranslateJSON(html []byte) []struct {
	Order     int    `json:"order"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
} {
	marker := []byte("var translate = ")
	idx := bytes.Index(html, marker)
	if idx < 0 {
		return nil
	}
	brace := bytes.IndexByte(html[idx:], '{')
	if brace < 0 {
		return nil
	}
	start := idx + brace
	dec := json.NewDecoder(bytes.NewReader(html[start:]))
	var obj translateObj
	if err := dec.Decode(&obj); err != nil {
		return nil
	}
	return obj.Translation
}

// extractMediaIDAndAudio 从 HTML 中提取音频 URL
// 形如 Webting_play.initPlayPage("https://api.frdic.com/api/v3/media/mp3/{mediaid}?type=mp3&cdn=auto&token=...")
var initPlayRe = regexp.MustCompile(`Webting_play\.initPlayPage\("([^"]+)"\)`)

func extractMediaIDAndAudio(html []byte) (mediaID, audioURL string) {
	m := initPlayRe.FindSubmatch(html)
	if m == nil {
		return "", ""
	}
	audioURL = string(m[1])
	// 从 URL 路径里取 mediaID
	if parts := strings.Split(audioURL, "/mp3/"); len(parts) == 2 {
		rest := parts[1]
		if q := strings.IndexByte(rest, '?'); q >= 0 {
			mediaID = rest[:q]
		}
	}
	return mediaID, audioURL
}

// parseStamp 把 "00:00.03" 转为秒数
func parseStamp(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) == 1 {
		// 纯秒数
		if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
			return v
		}
		return 0
	}
	min, err1 := strconv.ParseFloat(parts[0], 64)
	sec, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil {
		return 0
	}
	return min*60 + sec
}

// startStamp 把秒数转回 "MM:SS.ss" 形式（用于和 translation.timestamp 配对）
func startStamp(sec float64) string {
	total := int(sec * 100)
	m := total / 6000
	sRem := total - m*6000
	secInt := sRem / 100
	csInt := sRem % 100
	return fmt.Sprintf("%02d:%02d.%02d", m, secInt, csInt)
}
