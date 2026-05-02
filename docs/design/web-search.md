# P2 16 — Web Search / Fetch 技术方案

## 调研摘要

分析了 6 个主流 code agent 的 web search 实现：

| Agent | Search API | Fetch 方式 | 工具数量 | 备注 |
|-------|-----------|-----------|---------|------|
| **Claude Code** | 未公开（Anthropic 自有） | 后端代理抓取 | 2: `WebSearch` + `WebFetch` | 搜索成本算在订阅里 |
| **Cline** | 自有后端 | 后端代理 + Playwright | 3: `web_search` + `web_fetch` + `browser_action` | 15s 超时 |
| **Aider** | 无搜索 | Playwright + httpx → pandoc → MD | 1: `/web` 命令 | 纯 URL 抓取 |
| **Cursor** | MCP server 扩展 | MCP server | 取决于 MCP | 不内置 |
| **OpenHands** | Tavily + BrowserGym | Playwright 沙盒 | 浏览器动作空间 | Docker 隔离 |
| **GitHub Copilot** | 可能用 Bing | 无单独 fetch | `#web` 变量 | 单轮搜索 |

### 关键洞察

1. **search + fetch 两步模式**：所有 agent 都先搜 URL，再取内容
2. **`web_fetch` 的 `prompt` 参数是核心设计**：让 LLM 指导内容提取方向
3. **免费方案只能抓取 HTML**：DuckDuckGo API 只返回知识图谱，不适用中文查询
4. **HTML→文本是核心能力**，Go 标准扩展库 tokenizer 足够，不需要 Playwright

---

## 实现设计

### 零配置原则

- `web_search` → 抓取 Bing 搜索结果页 HTML（`www.bing.com/search?q=...&cc=cn&setmkt=zh-CN`），解析 `li.b_algo` 提取标题/URL/摘要
- `web_fetch` → HTTP GET + HTML→Markdown 转换
- **完全免费，零 API key，国内可用，不动 Config**

### `cc=cn&setmkt=zh-CN` 的作用

不加这两个参数时，Bing 按 IP 地理位置返回结果。非中文 IP 访问中文查询会拿到英文垃圾结果（如搜 "太白山" 返回 "Sign in to Gmail"）。加了强制走中国区，中文查询返回中文内容。

### 架构

```
LLM 需要查资料
       │
       ├──→ web_search(query)
       │       │
       │       ▼
       │    GET bing.com/search?q=...&cc=cn&setmkt=zh-CN
       │    (Chrome UA + zh-CN Accept-Language)
       │       │
       │       ▼
       │    解析 <li class="b_algo"> → title/url/snippet
       │       │
       │       ▼
       │    rune 安全截断 ≤2000 chars → LLM 上下文
       │
       ├──→ web_fetch(url, prompt?)
       │       │
       │       ▼
       │    URL 校验 → HTTP GET → HTML→Markdown
       │       │
       │       ▼
       │    rune 安全截断 ≤3000 chars → LLM 上下文
       │
       ▼
    LLM 基于结果继续推理
```

### 1. `web_search` — Bing HTML 抓取

**文件**：`bot/tools/tool_websearch.go`

- 请求：`GET https://www.bing.com/search?q=<query>&cc=cn&setmkt=zh-CN`
- Headers：Chrome 120 UA + `Accept-Language: zh-CN,zh;q=0.9,en;q=0.8`
- 超时：12s
- 响应上限：500KB

**解析**（`parseBingResults`）：
- `golang.org/x/net/html` tokenizer 遍历 DOM
- 找到 `<li class="b_algo">` → 提取子元素 `<h2>`（标题）、`<a>`（URL）、`<p>`/`<div>`（摘要）
- 输出 5 条结果，每条标题 + URL + 摘要

**格式化**：`truncateByRune` rune 安全截断，摘要 200 runes，整体 2000 runes

### 2. `web_fetch` — HTTP GET + HTML→MD

**文件**：`bot/tools/tool_webfetch.go`

- 超时：15s，最多 5 次重定向
- 响应上限：5MB
- Content-Type 检测：HTML 走 html2md，其他直接返回文本
- `prompt` 参数：关键词匹配提取相关段落（P0 不做 LLM 提取）

**安全校验**：
- 只允许 `http`/`https` 协议
- DNS 解析后检查 IP，禁止内网地址（RFC 1918 + loopback + link-local）
- 禁止 IPv6 私有地址（`fc00::/7`, `::1`, `fe80::/10`）

### 3. HTML→Markdown 转换器

**文件**：`bot/tools/html2md.go`

用 `golang.org/x/net/html` tokenizer，~160 行纯函数：

```
<script>, <style>, <svg>, <nav>, <footer>, <header>, <aside>, <noscript> → 跳过
<h1>-<h6>     → # ~ ######
<p>, <div>     → 双换行分隔
<a href="X">   → [text](X)
<code>         → `text`
<pre>          → ```text```
<li>           → - text
<strong>/<b>   → **text**
<em>/<i>       → *text*
<img>          → ![alt](src)
<br>           → 换行
连续 3+ 空行    → 压缩为 2 行
```

**测试**：`html2md_test.go` — 8 个用例覆盖标题/段落/链接/列表/图片/代码/粗斜体/script 跳过

### 4. UTF-8 安全截断

**`truncateByRune`**（`bot/tools/tool.go`）：

```go
func truncateByRune(s string, max int) string {
    runes := []rune(s)
    if len(runes) <= max { return s }
    return string(runes[:max])
}
```

Go 中 `s[:N]` 按字节切片，中文每个字 3 字节，切到中间就出 `�`。先 `[]rune(s)` 再截就不会撕裂多字节字符。`web_search` 和 `web_fetch` 的输出截断都用这个。

TUI 端同样问题，`tui/commands.go` 的 `truncate` 函数也改成 rune 安全。

### 5. 注册

`RegisterDefaults` 签名不变：

```go
func RegisterDefaults(r *Registry) {
    r.Register(&BashTool{})
    r.Register(&FileSystemTool{})
    r.Register(&GlobTool{})
    r.Register(&EditTool{})
    r.Register(&GrepTool{})
    r.Register(NewWebSearchTool())
    r.Register(NewWebFetchTool())
}
```

### 6. System Prompt 使用指引

```markdown
- 当需要查最新信息、文档或不确定的知识时，用 web_search 搜一次，
  从结果里挑最相关的 1-2 条，直接用 web_fetch 读取。
  不要反复换 query 搜索——先看完内容再判断是否需要补搜。
- 如果 web_fetch 返回空或失败，换搜索结果里的下一条链接重试，
  而不是重新搜索。
- web_fetch 的 prompt 参数用于指导内容提取。
```

### 7. Token 预算

| 输出 | 限制 | 策略 |
|------|------|------|
| `web_search` | ≤ 2000 runes | 最多 5 条结果，每条摘要 ≤ 200 runes |
| `web_fetch` | ≤ 3000 runes | HTML→MD 后 rune 安全截断 |
| `web_fetch` (有 prompt) | ≤ 3000 runes | 关键词匹配找相关段落，再截断 |

---

## 文件清单

| 文件 | 作用 |
|------|------|
| `bot/tools/tool_websearch.go` | Bing HTML 抓取 + 解析 + 格式化 |
| `bot/tools/tool_webfetch.go` | URL 抓取 + 安全校验 + html2md |
| `bot/tools/html2md.go` | HTML tokenizer → Markdown 转换器 |
| `bot/tools/html2md_test.go` | 8 个单元测试 |
| `bot/tools/tool.go` | `truncateByRune` + `RegisterDefaults` |
| `bot/prompt/system.md` | web_search/web_fetch 使用指引 |
| `tui/commands.go` | `truncate` rune 安全截断 |
| `go.mod` | 新增 `golang.org/x/net` |

---

## 风险与取舍

| 决策点 | 选择 | 理由 |
|--------|------|------|
| Search 后端 | Bing HTML 抓取 | 免费无 key，国内可用，加 `cc=cn&setmkt=zh-CN` 保证中文结果质量 |
| 抓取稳定性 | 接受 | HTML 结构变化可能需适配，但 Bing `b_algo` class 多年未变 |
| HTML 渲染 | 不用 Playwright | 终端工具不需要渲染 SPA，文档站和服务端页面 SSR 够用 |
| html2md 依赖 | `golang.org/x/net/html` | Go 生态标准扩展库，tokenizer 够用 |
| prompt 提取 | P0 关键词匹配降级 | 不做 LLM 二次提取，html2md 拿全文给 LLM 自己判断 |
| 配置 | 零新增 | Bing 免费无需 key |
| 缓存 | P0 不做 | agent 场景重复 fetch 概率低 |
