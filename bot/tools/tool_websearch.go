package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type WebSearchTool struct {
	client *http.Client
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{client: NewToolHTTPClient(30 * time.Second)}
}

func (t *WebSearchTool) Name() string                                   { return "web_search" }
func (t *WebSearchTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeParallel }
func (t *WebSearchTool) DangerLevel(map[string]interface{}) DangerLevel    { return LevelSafe }

func (t *WebSearchTool) Description() string {
	return fmt.Sprintf(
		`搜索网页获取最新信息。可指定结果数量（默认8，最大15）。
提示：搜索今年（%d年）的内容时请在query中包含年份以获得更准确的结果。`,
		time.Now().Year(),
	)
}

func (t *WebSearchTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "query", Type: "string", Required: true, Description: "搜索查询词"},
		{Name: "numResults", Type: "number", Required: false, Description: "返回数量，默认8，最大15"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("缺少 query 参数")
	}
	n := 8
	if v, ok := args["numResults"].(float64); ok && v > 0 {
		n = int(v)
		if n > 15 {
			n = 15
		}
	}
	// Try Exa first (free tier works without API key), fall back to Bing.
	if s, err := searchExa(ctx, query, n); err == nil {
		return s, nil
	}
	return searchBing(ctx, query, n)
}

// --- Exa MCP (JSON-RPC over SSE) ---

func exaEndpoint() string {
	u := "https://mcp.exa.ai/mcp"
	if k := os.Getenv("EXA_API_KEY"); k != "" {
		u += "?exaApiKey=" + url.QueryEscape(k)
	}
	return u
}

func searchExa(ctx context.Context, query string, n int) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "web_search_exa",
			"arguments": map[string]interface{}{
				"query":                query,
				"numResults":           n,
				"livecrawl":            "fallback",
				"type":                 "auto",
				"contextMaxCharacters": 10000,
			},
		},
	})

	req, _ := http.NewRequestWithContext(ctx, "POST", exaEndpoint(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := NewToolHTTPClient(30*time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("exa: HTTP %d — %s", resp.StatusCode, string(b))
	}
	return parseExaSSE(resp.Body)
}

func parseExaSSE(r io.Reader) (string, error) {
	scan := bufio.NewScanner(r)
	for scan.Scan() {
		line := scan.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var v struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if json.Unmarshal([]byte(line[6:]), &v) != nil {
			continue
		}
		if len(v.Result.Content) > 0 && v.Result.Content[0].Text != "" {
			return TruncateByRune(v.Result.Content[0].Text, 6000), nil
		}
	}
	return "", scan.Err()
}

// --- Bing fallback ---

type bingResult struct{ title, url, snippet string }

func searchBing(ctx context.Context, query string, n int) (string, error) {
	u := "https://www.bing.com/search?q=" + url.QueryEscape(query) + "&count=15"
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := NewToolHTTPClient(12*time.Second).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 500<<10))
	results := parseBing(string(raw))
	if len(results) == 0 {
		return "未找到相关结果", nil
	}
	if len(results) > n {
		results = results[:n]
	}
	return formatBing(results, query), nil
}

func parseBing(s string) []bingResult {
	var out []bingResult
	z := html.NewTokenizer(strings.NewReader(s))

	var inRes, inH2, inP bool
	var cur bingResult
	depth := 0
	seen := map[string]bool{}

	for {
		switch z.Next() {
		case html.ErrorToken:
			if cur.title != "" || cur.snippet != "" {
				out = append(out, cur)
			}
			// Deduplicate by title/url.
			deduped := make([]bingResult, 0, len(out))
			for _, r := range out {
				k := r.title
				if k == "" {
					k = r.url
				}
				if !seen[k] {
					seen[k] = true
					deduped = append(deduped, r)
				}
			}
			return deduped

		case html.StartTagToken:
			tag, hasAttr := z.TagName()
			a := atom.Lookup(tag)

			// Detect search result <li class="b_algo|b_ad|b_ans">
			if a == atom.Li && hasAttr {
				for {
					k, v, more := z.TagAttr()
					if string(k) == "class" {
						cv := string(v)
						if strings.Contains(cv, "b_algo") || strings.Contains(cv, "b_ad") || strings.Contains(cv, "b_ans") {
							if inRes {
								out = append(out, cur)
							}
							cur = bingResult{}
							inRes = true
							depth = 0
							break
						}
					}
					if !more {
						break
					}
				}
				continue
			}

			if !inRes {
				continue
			}
			depth++
			switch a {
			case atom.H2:
				inH2 = true
			case atom.A:
				if inH2 && hasAttr {
					for {
						k, v, more := z.TagAttr()
						if string(k) == "href" {
							cur.url = string(v)
							break
						}
						if !more {
							break
						}
					}
				}
			case atom.P:
				inP = true
			}

		case html.EndTagToken:
			tag, _ := z.TagName()
			a := atom.Lookup(tag)
			if a == atom.H2 {
				inH2 = false
			}
			if a == atom.P {
				inP = false
			}
			if inRes {
				depth--
				if depth <= 0 {
					inRes = false
				}
			}

		case html.TextToken:
			if inH2 {
				cur.title += strings.TrimSpace(string(z.Text()))
			} else if inP {
				cur.snippet += strings.TrimSpace(string(z.Text())) + " "
			}
		}
	}
}

func formatBing(results []bingResult, query string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "搜索: %s", query)
	for i, r := range results {
		if i >= 8 || r.title == "" {
			continue
		}
		fmt.Fprintf(&b, "\n\n%d. %s\n   %s", i+1, r.title, r.url)
		if s := strings.TrimSpace(r.snippet); s != "" {
			fmt.Fprintf(&b, "\n   %s", TruncateByRune(s, 200))
		}
	}
	return TruncateByRune(b.String(), 3000)
}
