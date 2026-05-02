package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type WebSearchTool struct {
	client *http.Client
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		client: &http.Client{Timeout: 12 * time.Second},
	}
}

func (t *WebSearchTool) Name() string                                 { return "web_search" }
func (t *WebSearchTool) DangerLevel(map[string]interface{}) DangerLevel { return LevelSafe }

func (t *WebSearchTool) Description() string {
	return "使用 Bing 搜索网页获取最新信息"
}

func (t *WebSearchTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "query", Type: "string", Required: true, Description: "搜索查询词"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("缺少 query 参数")
	}

	searchURL := "https://www.bing.com/search?q=" + url.QueryEscape(query) + "&cc=cn&setmkt=zh-CN"

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("搜索请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500<<10))
	if err != nil {
		return "", fmt.Errorf("读取搜索结果失败: %v", err)
	}

	results := parseBingResults(string(body))
	if len(results) == 0 {
		return "未找到相关结果", nil
	}

	return formatSearchResults(results, query), nil
}

type searchResult struct {
	title   string
	url     string
	snippet string
}

func parseBingResults(htmlStr string) []searchResult {
	var results []searchResult
	z := html.NewTokenizer(strings.NewReader(htmlStr))

	var inResult, inTitle, inSnippet bool
	var current searchResult
	depth := 0

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}

		switch tt {
		case html.StartTagToken:
			tagName, hasAttr := z.TagName()
			a := atom.Lookup(tagName)

			if a == atom.Li && hasAttr {
				for {
					k, v, more := z.TagAttr()
					if string(k) == "class" && strings.Contains(string(v), "b_algo") {
						if inResult {
							results = append(results, current)
						}
						current = searchResult{}
						inResult = true
						depth = 0
						break
					}
					if !more {
						break
					}
				}
			}

			if inResult {
				depth++
				if a == atom.H2 {
					inTitle = true
				} else if a == atom.P || (a == atom.Div && inResult) {
					inSnippet = true
				} else if a == atom.A && hasAttr && inTitle {
					for {
						k, v, more := z.TagAttr()
						if string(k) == "href" {
							current.url = string(v)
							break
						}
						if !more {
							break
						}
					}
				}
			}

		case html.EndTagToken:
			tagName, _ := z.TagName()
			a := atom.Lookup(tagName)
			if a == atom.H2 {
				inTitle = false
			} else if a == atom.P {
				inSnippet = false
			}

			if inResult {
				depth--
				if depth <= 0 {
					if current.title != "" || current.snippet != "" {
						results = append(results, current)
						current = searchResult{}
					}
					inResult = false
				}
			}

		case html.TextToken:
			if inTitle {
				text := strings.TrimSpace(string(z.Text()))
				current.title += text
			} else if inSnippet {
				text := strings.TrimSpace(string(z.Text()))
				if text != "" {
					current.snippet += text + " "
				}
			}
		}
	}

	if current.title != "" || current.snippet != "" {
		results = append(results, current)
	}

	return results
}

func formatSearchResults(results []searchResult, query string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("搜索: %s\n", query))

	count := 0
	for _, r := range results {
		if r.title == "" || count >= 5 {
			continue
		}

		snippet := truncateByRune(strings.TrimSpace(r.snippet), 200)

		b.WriteString(fmt.Sprintf("\n%d. %s\n", count+1, r.title))
		if r.url != "" {
			b.WriteString(fmt.Sprintf("   %s\n", r.url))
		}
		if snippet != "" {
			b.WriteString(fmt.Sprintf("   %s\n", snippet))
		}
		count++
	}

	if count == 0 {
		return "未找到相关结果"
	}

	return truncateByRune(b.String(), 2000)
}
