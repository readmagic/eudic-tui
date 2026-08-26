package parser

import (
	"os"
	"testing"
)

func TestParseMediaList(t *testing.T) {
	html, err := os.ReadFile("../../testdata/sample_list.html")
	if err != nil {
		t.Fatalf("读取 sample_list.html 失败: %v", err)
	}
	list, err := ParseMediaList(html)
	if err != nil {
		t.Fatalf("ParseMediaList 失败: %v", err)
	}
	if len(list) != 10 {
		t.Fatalf("期望 10 条，得到 %d 条", len(list))
	}
	first := list[0]
	if first.MediaID != "5ad1a41f-51d0-4465-bc90-2370b2b9cf76" {
		t.Errorf("MediaID = %q, 期望 5ad1a41f...", first.MediaID)
	}
	if first.Title != "全国英语等级考试三级历年真题试卷（六）" {
		t.Errorf("Title = %q", first.Title)
	}
	if first.FileTime != "26-08-26 10:26:29" {
		t.Errorf("FileTime = %q", first.FileTime)
	}
	if first.Status != "处理完成" {
		t.Errorf("Status = %q", first.Status)
	}
	if first.DetailToken == "" {
		t.Errorf("DetailToken 为空")
	}
	if first.DetailToken[:3] != "QYN" {
		t.Errorf("DetailToken 开头 = %q, 期望 QYN...", first.DetailToken[:3])
	}
}

func TestParseDetail(t *testing.T) {
	html, err := os.ReadFile("../../testdata/sample_detail.html")
	if err != nil {
		t.Fatalf("读取 sample_detail.html 失败: %v", err)
	}
	d, err := ParseDetail(html)
	if err != nil {
		t.Fatalf("ParseDetail 失败: %v", err)
	}
	if d.MediaID != "5ad1a41f-51d0-4465-bc90-2370b2b9cf76" {
		t.Errorf("MediaID = %q", d.MediaID)
	}
	if d.Title == "" {
		t.Errorf("Title 为空")
	}
	if !contains(d.Title, "全国英语等级考试三级历年真题试卷") {
		t.Errorf("Title = %q, 期望包含全国英语等级考试...", d.Title)
	}
	if len(d.Sentences) == 0 {
		t.Fatalf("Sentences 为空")
	}
	if len(d.Sentences) < 50 {
		t.Errorf("Sentences 数量 = %d, 期望 >= 50", len(d.Sentences))
	}
	first := d.Sentences[0]
	if first.ID != "J_00:00.03" {
		t.Errorf("first.ID = %q", first.ID)
	}
	if first.Start != 0.03 {
		t.Errorf("first.Start = %v, 期望 0.03", first.Start)
	}
	if first.End != 7.76 {
		t.Errorf("first.End = %v, 期望 7.76", first.End)
	}
	if first.Text == "" {
		t.Errorf("first.Text 为空")
	}
	if d.AudioURL == "" {
		t.Errorf("AudioURL 为空")
	}
	if !contains(d.AudioURL, "api.frdic.com/api/v3/media/mp3/") {
		t.Errorf("AudioURL = %q, 期望包含 api.frdic.com...", d.AudioURL)
	}
	// 验证翻译配对
	transCount := 0
	for _, s := range d.Sentences {
		if s.Translation != "" {
			transCount++
		}
	}
	if transCount == 0 {
		t.Errorf("没有任何句子被配对到翻译")
	}
}

func TestStartStamp(t *testing.T) {
	cases := []struct {
		sec  float64
		want string
	}{
		{0.03, "00:00.03"},
		{7.76, "00:07.76"},
		{140.91, "02:20.91"},
	}
	for _, c := range cases {
		got := startStamp(c.sec)
		if got != c.want {
			t.Errorf("startStamp(%v) = %q, 期望 %q", c.sec, got, c.want)
		}
	}
}

func TestParseStamp(t *testing.T) {
	cases := []struct {
		s    string
		want float64
	}{
		{"00:00.03", 0.03},
		{"00:07.76", 7.76},
		{"02:20.91", 140.91},
	}
	for _, c := range cases {
		got := parseStamp(c.s)
		if got != c.want {
			t.Errorf("parseStamp(%q) = %v, 期望 %v", c.s, got, c.want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
