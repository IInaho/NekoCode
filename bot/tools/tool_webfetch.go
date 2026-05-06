package tools

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type WebFetchTool struct {
	client *http.Client
}

func NewWebFetchTool() *WebFetchTool {
	c := NewToolHTTPClient(15 * time.Second)
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("重定向次数过多")
		}
		return nil
	}
	return &WebFetchTool{client: c}
}

func (t *WebFetchTool) Name() string        { return "web_fetch" }
	func (t *WebFetchTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeParallel }
func (t *WebFetchTool) DangerLevel(map[string]interface{}) DangerLevel { return LevelSafe }

func (t *WebFetchTool) Description() string {
	return "抓取网页内容并转换为文本，可用于读取文档、API 参考等"
}

func (t *WebFetchTool) Parameters() []Parameter {
	return []Parameter{
		{Name: "url", Type: "string", Required: true, Description: "要抓取的网页 URL"},
		{Name: "prompt", Type: "string", Required: false, Description: "内容提取指导，如'提取 API 参数说明'"},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	rawURL, ok := args["url"].(string)
	if !ok || strings.TrimSpace(rawURL) == "" {
		return "", fmt.Errorf("缺少 url 参数")
	}

	if err := validateURL(rawURL); err != nil {
		return "", fmt.Errorf("URL 校验失败: %v", err)
	}

	prompt, _ := args["prompt"].(string)

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %v", err)
	}
	req.Header.Set("User-Agent", "PrimusBot/1.0")
	req.Header.Set("Accept", "text/html,text/plain,*/*")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %v", err)
	}

	contentType := resp.Header.Get("Content-Type")
	var content string
	if strings.Contains(contentType, "text/html") {
		content = html2md(string(body))
	} else {
		content = string(body)
	}

	content = StripAnsi(content)

	if content == "" {
		return "页面内容为空", nil
	}

	if prompt != "" {
		content = extractRelevant(content, prompt)
	}

	content = TruncateByRune(content, 3000)
	return content, nil
}

func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("无效 URL: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("只允许 http/https 协议")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("缺少主机名")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("禁止访问内网地址")
		}
	} else {
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("DNS 解析失败: %v", err)
		}
		for _, ip := range ips {
			if isPrivateIP(ip) {
				return fmt.Errorf("禁止访问内网地址")
			}
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"169.254.0.0/16",
		"fc00::/7",
	}
	for _, cidr := range privateBlocks {
		_, block, _ := net.ParseCIDR(cidr)
		if block != nil && block.Contains(ip) {
			return true
		}
	}
	return false
}

func extractRelevant(content, prompt string) string {
	keywords := strings.Fields(prompt)
	if len(keywords) == 0 {
		return content
	}

	paragraphs := strings.Split(content, "\n\n")
	var relevant []string
	for _, p := range paragraphs {
		pLower := strings.ToLower(p)
		for _, kw := range keywords {
			if strings.Contains(pLower, strings.ToLower(kw)) {
				relevant = append(relevant, p)
				break
			}
		}
	}
	if len(relevant) == 0 {
		return content
	}
	return strings.Join(relevant, "\n\n")
}
