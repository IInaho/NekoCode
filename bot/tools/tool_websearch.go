package tools

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type WebSearchTool struct {
	client *http.Client
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{
		client: NewToolHTTPClient(12 * time.Second),
	}
}

func (t *WebSearchTool) Name() string                                  { return "web_search" }
	func (t *WebSearchTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeParallel }
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

	searchURL := "https://www.bing.com/search?q=" + url.QueryEscape(query) + "&setmkt=zh-CN&count=15"

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

	filtered := filterRelevant(results, query)
	if len(filtered) == 0 {
		filtered = results
	}

	return formatSearchResults(filtered, query), nil
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
				} else if a == atom.P || (a == atom.Div && inResult) {
					inSnippet = true
				}
			} else {
				// Also match b_ad results (sidebar/news results, class differs from b_algo).
				if a == atom.Li && hasAttr {
					for {
						k, v, more := z.TagAttr()
						if string(k) == "class" && (strings.Contains(string(v), "b_ad") || strings.Contains(string(v), "b_ans")) {
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

	return deduplicate(results)
}

func deduplicate(results []searchResult) []searchResult {
	seen := make(map[string]bool)
	var out []searchResult
	for _, r := range results {
		key := r.title
		if key == "" {
			key = r.url
		}
		if !seen[key] {
			seen[key] = true
			out = append(out, r)
		}
	}
	return out
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana)
}

func tokenize(s string) []string {
	var tokens []string
	seen := make(map[string]bool)

	// Split on whitespace first, then bigram each segment.
	for _, seg := range strings.Fields(s) {
		seg = strings.ToLower(seg)
		runes := []rune(seg)

		// For short ASCII segments, keep as-is.
		asciiOnly := true
		for _, r := range runes {
			if r > 127 {
				asciiOnly = false
				break
			}
		}
		if asciiOnly {
			if len(runes) >= 2 && !seen[seg] {
				tokens = append(tokens, seg)
				seen[seg] = true
			}
			continue
		}

		// For CJK segments, generate character bigrams.
		if len(runes) == 1 {
			if !seen[seg] {
				tokens = append(tokens, seg)
				seen[seg] = true
			}
			continue
		}
		for i := 0; i < len(runes)-1; i++ {
			bg := string(runes[i : i+2])
			if !seen[bg] {
				tokens = append(tokens, bg)
				seen[bg] = true
			}
		}
	}
	return tokens
}

func filterRelevant(results []searchResult, query string) []searchResult {
	qTokens := tokenize(query)
	if len(qTokens) == 0 {
		return results
	}

	type scoredItem struct {
		r searchResult
		s int
	}
	var list []scoredItem
	for _, r := range results {
		text := strings.ToLower(r.title + " " + r.snippet)
		s := 0
		for _, t := range qTokens {
			if strings.Contains(text, t) {
				s++
			}
		}
		if s > 0 {
			list = append(list, scoredItem{r, s})
		}
	}

	// If filtering removed everything, return original.
	if len(list) == 0 {
		return results
	}

	sort.Slice(list, func(i, j int) bool { return list[i].s > list[j].s })

	out := make([]searchResult, len(list))
	for i, it := range list {
		out[i] = it.r
	}
	return out
}

func formatSearchResults(results []searchResult, query string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("搜索: %s\n", query))

	count := 0
	for _, r := range results {
		if r.title == "" || count >= 8 {
			continue
		}

		snippet := TruncateByRune(strings.TrimSpace(r.snippet), 200)

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

	return TruncateByRune(b.String(), 3000)
}
