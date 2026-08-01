//go:build ignore
package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-shiori/go-readability"
)

func main() {
	urls := []string{
		"https://www.cnblogs.com/coco1s/p/14230585",
		"https://www.cnblogs.com/mfrank/p/10496876.html",
		"https://www.cnblogs.com/stulzq/p/18090077",
		"https://www.cnblogs.com/myvin/p/21346634",
	}
	for _, u := range urls {
		a, err := readability.FromURL(u, 30*time.Second, readability.RequestWith(func(r *http.Request) {
			r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/126.0.0.0")
		}))
		if err != nil {
			fmt.Printf("%-55s ERROR: %v\n", u, err)
			continue
		}
		fmt.Printf("%-55s 标题:%q 长度:%d字\n", u, a.Title, len([]rune(a.TextContent)))
	}
}
