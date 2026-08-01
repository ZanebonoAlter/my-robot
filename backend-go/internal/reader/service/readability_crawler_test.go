package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// repeatString 生成指定长度的正文，便于构造测试用例。
func repeatString(s string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(s)
	}
	return b.String()
}

// TestIsUsableArticle_QualityThreshold 覆盖调研实测的四类场景：
// 博客园合格正文、36氪太短、假成功垃圾词过量、边界合格。
func TestIsUsableArticle_QualityThreshold(t *testing.T) {
	// 5814 字干净正文（博客园实测，0 垃圾词）
	longClean := repeatString("这是正文段落。", 1000) // 7000 字, 0 垃圾词

	// 160 字页脚（36氪实测，3 垃圾词：备案+ICP+版权）
	shortJunk := "本站由阿里云提供计算与安全服务 举报电话 京ICP备12031756号 版权所有"

	// 650 字但含 3 垃圾词（假成功：正文够长但混入页脚垃圾）
	mixedJunk := repeatString("正文内容。", 100) + " 京ICP备12031756号 版权所有 举报电话"

	// 650 字只含 1 垃胶词（边界合格：正文为主，偶然命中一个词）
	edgeOK := repeatString("正文内容。", 100) + "版权"

	tests := []struct {
		name string
		md   string
		want bool
	}{
		{"博客园长正文_0垃圾词_合格", longClean, true},
		{"36氪短页脚_3垃圾词_不合格", shortJunk, false},
		{"假成功_3垃圾词_不合格", mixedJunk, false},
		{"边界_1垃圾词_合格", edgeOK, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUsableArticle(tt.md)
			if got != tt.want {
				t.Errorf("isUsableArticle() = %v, want %v (len=%d)", got, tt.want, len([]rune(tt.md)))
			}
		})
	}
}

// TestReadabilityCrawler_ScrapePage_SSR 验证 ReadabilityCrawler 对本地 SSR 页面的
// 端到端提取：返回中立 ScrapeResult，Source 标记为 readability，Markdown 非空。
func TestReadabilityCrawler_ScrapePage_SSR(t *testing.T) {
	// 构造一个含足够正文的 SSR 文章页（模拟博客园这类服务端渲染站点）。
	body := repeatString("<p>这是文章正文段落，包含足够多的内容用于通过质量校验阈值。</p>", 50)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>测试文章</title></head>
		<body><article>` + body + `</article></body></html>`))
	}))
	defer srv.Close()

	c := NewReadabilityCrawler()
	res, err := c.ScrapePage(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("ScrapePage failed: %v", err)
	}
	if res.Source != "readability" {
		t.Errorf("Source = %q, want %q", res.Source, "readability")
	}
	if len([]rune(res.Markdown)) < minUsableContentRunes {
		t.Errorf("Markdown too short: %d runes, want >= %d", len([]rune(res.Markdown)), minUsableContentRunes)
	}
	if res.Title != "测试文章" {
		t.Errorf("Title = %q, want %q", res.Title, "测试文章")
	}
}

// TestReadabilityCrawler_ScrapePage_SatisfiesInterface 编译期验证
// ReadabilityCrawler 满足 Crawler 接口。
func TestReadabilityCrawler_ScrapePage_SatisfiesInterface(t *testing.T) {
	var _ Crawler = (*ReadabilityCrawler)(nil)
}
