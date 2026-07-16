//go:build ignore
package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

// 垃圾特征词：页脚/备案/版权/导航，出现说明提取到的是壳不是正文。
var junkSignals = []string{"备案", "ICP", "版权", "举报", "Copyright", "©", "登录", "注册", "All Rights"}

type probe struct{ name, url string }

func main() {
	items := []probe{
		{"博客园真实文章", "https://www.cnblogs.com/myvin/p/21346634"},
		{"36氪-SPA?", "https://36kr.com/p/2856475891828488"},
		{"掘金-SPA?", "https://juejin.cn/post/7395144423331516479"},
		{"机器之心", "https://www.jiqizhixin.com/articles/2024-08-01"},
		{"智源hub", "https://hub.baai.ac.cn/"},
	}
	for _, it := range items {
		a, err := readability.FromURL(it.url, 30*time.Second, readability.RequestWith(func(r *http.Request) {
			r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/126.0.0.0")
		}))
		if err != nil {
			fmt.Printf("%-16s ❌ ERROR: %v\n", it.name, err)
			continue
		}
		txt := a.TextContent
		junkHits := 0
		for _, j := range junkSignals {
			if strings.Contains(txt, j) {
				junkHits++
			}
		}
		verdict := "✅ 正文干净"
		if len([]rune(txt)) < 500 {
			verdict = "⚠️ 太短（疑似空壳/SPA）"
		} else if junkHits >= 3 {
			verdict = "⚠️ 垃圾占比高（页脚/备案），疑似 SPA 假成功"
		}
		// 打印前150字
		preview := strings.TrimSpace(txt)
		if len([]rune(preview)) > 150 {
			preview = string([]rune(preview)[:150])
		}
		preview = strings.ReplaceAll(preview, "\n", " ")
		fmt.Printf("%-16s 长度:%-5d 垃圾命中:%d/9  %s\n", it.name, len([]rune(txt)), junkHits, verdict)
		fmt.Printf("                 预览: %s…\n\n", preview)
	}
}
