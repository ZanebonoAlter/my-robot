// readability-spike: 调研纯 Go 正文提取方案（readability + html-to-markdown）
// 对一批代表性 URL 同时跑 shiori（旧）和 readeck（新）两套 readability，
// 统计标题、正文长度、提取耗时、是否拿到合格正文，判定能否替代 Firecrawl。
package main

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/go-shiori/go-readability"

	// 用别名区分新旧两套 readability
	readeck "codeberg.org/readeck/go-readability/v2"
)

// minContentChars 是"合格正文"的阈值：低于此长度视为提取失败（空壳/翻车）。
// 与后端 ContentCompletionService.IsContentIncomplete 的 200 字阈值对齐参考，这里取更保守的 500。
const minContentChars = 500

// 模拟桌面浏览器 UA，避免被简单 UA 黑名单挡掉。
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
	"(KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// 抓取超时，与后端 firecrawl 默认 60s 对齐（readability 本身快得多，给宽裕值）。
const fetchTimeout = 30 * time.Second

// sampleURLs 覆盖用户提到的典型订阅源：博客园、智源等。
var sampleURLs = []struct {
	Name string
	URL  string
}{
	{"博客园-首页", "https://www.cnblogs.com/"},
	{"博客园-文章1", "https://www.cnblogs.com/coco1s/p/18235455"},
	{"博客园-文章2", "https://www.cnblogs.com/stulzq/p/18090077"},
	{"智源社区-首页", "https://event.baai.ac.cn/"},
	{"智源研究院", "https://www.baai.ac.cn/"},
	{"智源BAAI-文章", "https://www.baai.ac.cn/news/1500.html"},
	{"36氪-文章", "https://36kr.com/p/2856475891828488"},
	{"少数派-文章", "https://sspai.com/post/73254"},
	{"掘金-文章", "https://juejin.cn/post/7395144423331516479"},
	{"机器之心", "https://www.jiqizhixin.com/articles/2024-08-01"},
}

type result struct {
	Name      string
	URL       string
	Lib       string // "shiori" 或 "readeck"
	Success   bool   // 是否拿到合格正文（> minContentChars）
	Title     string
	MDLength  int
	HasImage  bool
	Elapsed   time.Duration
	FetchErr  string
	MDPreview string // 正文前 200 字预览
}

func main() {
	conv := md.NewConverter("", true, nil)

	var results []result

	for _, s := range sampleURLs {
		// shiori（旧版）
		r1 := runShiori(s.Name, s.URL, conv)
		results = append(results, r1)
		printResult(r1)

		// readeck（新版继任者）
		r2 := runReadeck(s.Name, s.URL, conv)
		results = append(results, r2)
		printResult(r2)
	}

	printSummary(results)
}

// withUA 设置桌面浏览器 User-Agent，避免被简单 UA 黑名单挡掉。
func withUA(r *http.Request) {
	r.Header.Set("User-Agent", userAgent)
}

// runShiori 跑 go-shiori/go-readability（旧版，字段式 Article）。
func runShiori(name, url string, conv *md.Converter) result {
	r := result{Name: name, URL: url, Lib: "shiori"}
	start := time.Now()
	article, err := readability.FromURL(url, fetchTimeout, readability.RequestWith(withUA))
	if err != nil {
		r.FetchErr = err.Error()
		r.Elapsed = time.Since(start)
		return r
	}
	htmlContent := article.Content
	mdStr, convErr := conv.ConvertString(htmlContent)
	if convErr == nil {
		r.MDLength = len([]rune(mdStr))
		if r.MDLength >= minContentChars {
			r.Success = true
		}
		r.MDPreview = truncate(strings.TrimSpace(mdStr), 200)
	}
	r.Title = article.Title
	r.HasImage = article.Image != ""
	r.Elapsed = time.Since(start)
	return r
}

// runReadeck 跑 codeberg.org/readeck/go-readability/v2（新版，方法式 Article + RenderHTML）。
func runReadeck(name, url string, conv *md.Converter) result {
	r := result{Name: name, URL: url, Lib: "readeck"}
	start := time.Now()
	article, err := readeck.FromURL(url, fetchTimeout, readeck.RequestWith(withUA))
	if err != nil {
		r.FetchErr = err.Error()
		r.Elapsed = time.Since(start)
		return r
	}
	// readeck 用 RenderHTML 写入 io.Writer，再转 markdown。
	var buf bytes.Buffer
	if err := article.RenderHTML(&buf); err != nil {
		r.FetchErr = "render html: " + err.Error()
		r.Elapsed = time.Since(start)
		return r
	}
	mdStr, convErr := conv.ConvertString(buf.String())
	if convErr == nil {
		r.MDLength = len([]rune(mdStr))
		if r.MDLength >= minContentChars {
			r.Success = true
		}
		r.MDPreview = truncate(strings.TrimSpace(mdStr), 200)
	}
	r.Title = article.Title()
	r.HasImage = article.ImageURL() != ""
	r.Elapsed = time.Since(start)
	return r
}

func truncate(s string, n int) string {
	if len([]rune(s)) > n {
		return string([]rune(s)[:n])
	}
	return s
}

func printResult(r result) {
	status := "✗ FAIL"
	if r.Success {
		status = "✓ OK"
	}
	fmt.Printf("── %-22s [%s] %s (%dms, %d字)\n", r.Name, r.Lib, status, r.Elapsed.Milliseconds(), r.MDLength)
	if r.FetchErr != "" {
		fmt.Printf("   错误: %s\n", r.FetchErr)
	}
	if r.MDPreview != "" {
		fmt.Printf("   预览: %s…\n", strings.ReplaceAll(r.MDPreview, "\n", " "))
	}
}

func printSummary(results []result) {
	// 按站点聚合，统计每个站至少有一个库成功的比例
	siteOutcomes := map[string]bool{}
	for _, r := range results {
		if r.Success {
			siteOutcomes[r.Name] = true
		}
	}
	totalSites := len(sampleURLs)
	okSites := len(siteOutcomes)

	// 按库分别统计成功率
	libOK := map[string]int{}
	libTotal := map[string]int{}
	for _, r := range results {
		libTotal[r.Lib]++
		if r.Success {
			libOK[r.Lib]++
		}
	}

	fmt.Println("\n════════════════════════════════════════════════")
	fmt.Println("                 调研结论汇总")
	fmt.Println("════════════════════════════════════════════════")
	fmt.Printf("测试站点数: %d | 至少一库成功的站点: %d (%.0f%%)\n",
		totalSites, okSites, pct(okSites, totalSites))
	var libs []string
	for lib := range libTotal {
		libs = append(libs, lib)
	}
	sort.Strings(libs)
	for _, lib := range libs {
		fmt.Printf("  %-8s: %d/%d 成功 (%.0f%%)\n", lib, libOK[lib], libTotal[lib], pct(libOK[lib], libTotal[lib]))
	}
	fmt.Println("──────────────────────────────────────────────────")
	if okSites >= int(float64(totalSites)*0.7) {
		fmt.Println("结论: readability 主力方案可行，覆盖大多数站点 → 可开 change")
	} else {
		fmt.Println("结论: 覆盖率不足，需评估是否补浏览器兜底层")
	}
	fmt.Println("════════════════════════════════════════════════")
}

func pct(ok, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(ok) / float64(total) * 100
}
