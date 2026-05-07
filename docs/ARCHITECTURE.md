# PrimusBot 架构文档

## 项目概述

PrimusBot 是一个基于 Go 的终端 AI 助手，使用 Bubble Tea v2 构建 TUI，支持多 LLM provider（OpenAI / Anthropic / GLM / DeepSeek），具备 Agent 循环、Native Function Calling、工具执行、权限确认和上下文管理机制。

## 目录结构

```
primusbot/
├── main.go                     # 入口：无参→TUI 交互模式，有参→单次 CLI 模式
├── bot/                        # 核心组装层
│   ├── bot.go                  #   Bot 结构体、依赖注入、公开 API
│   ├── command.go              #   斜杠命令系统（/help /clear /stats /summarize /config）
│   ├── config.go               #   配置管理 (~/.primusbot/config.json)
│   ├── project_context.go      #   工作目录树构建（并发行数统计）
│   ├── prompt/system.md        #   [embed] 角色 system prompt + 工具使用规则
│   ├── types/types.go          #   共享类型：ConfirmRequest / ConfirmFunc / PhaseFunc
│   ├── extensions/             #   插件注册
│   │   └── extensions.go       #     Extension 接口：Tools() + Commands()
│   ├── agent/                  #   Agent 循环
│   │   ├── agent.go            #     Agent 结构体、ShouldStop、ContextTransform、token 统计
│   │   ├── run.go              #     主循环 (Reason→Execute→Feedback) + BTW 中断处理
│   │   ├── reasoner.go         #     LLM 调用、工具调用解析、流式 token 处理
│   │   ├── executor.go         #     批量工具调度（串行/并行）、DangerLevel 检查、确认回调
│   │   └── retry.go            #     LLM 调用指数退避重试（0.5s→8s，最多4次）
│   └── tools/                  #   工具系统
│       ├── tool.go             #     Tool 接口、Registry、ExecutionMode、ParseCall、StripAnsi
│       ├── tool_bash.go        #     BashTool — Shell 命令执行 + 四级危险分级 + ANSI 清理
│       ├── tool_filesystem.go  #     FileSystemTool — 文件读写列表 + 读缓存去重
│       ├── tool_glob.go        #     GlobTool — 文件模式匹配
│       ├── tool_grep.go        #     GrepTool — ripgrep 内容搜索
│       ├── tool_edit.go        #     EditTool — 精确字符串替换 + diff 输出
│       ├── tool_webfetch.go    #     WebFetchTool — HTTP GET + HTML→Markdown + DNS 安全校验
│       ├── tool_websearch.go   #     WebSearchTool — Exa AI MCP 优先 + Bing HTML 降级
│       └── html2md.go          #     HTML→Markdown 转换器 + collapseBlankLines
│   └── ctxmgr/                 # 上下文管理（按职责拆分 4 文件）
│       ├── manager.go          #   Manager struct + Build() 上下文组装 + 统计接口
│       ├── storage.go          #   消息存取：Add/AddToolResult/AddToolResultsBatch/Clear
│       ├── token.go            #   Token 估算：语言感知（CJK ~1.5/token, ASCII ~4/token）
│       └── summarize.go        #   结构化摘要：NeedsSummarization / Summarize / BuildPrompt
├── llm/                        # LLM 抽象层
│   ├── llm.go                  #   LLM 接口、Message/Response/ToolDef 等核心类型
│   ├── openai_compat.go        #   OpenAI 兼容实现（OpenAI / GLM / DeepSeek）
│   ├── anthropic.go            #   Anthropic 实现（含 tool_use/tool_result 双向转换 + SSE 流式）
│   └── event_reader.go         #   SSE 流解析器
├── tui/                        # 终端 UI（Bubble Tea v2）
│   ├── tui.go                  #   Run() 入口：Warmup + 创建 Program
│   ├── model.go                #   Model 定义、BotInterface、状态机、Confirm/Phase 回调
│   ├── update.go               #   Update 消息分发
│   ├── handlers.go             #   spinner/done/confirm/keyPress 四大 handler
│   ├── agent_runner.go         #   startChat/startAgent：命令分发 + Agent goroutine
│   ├── helpers.go              #   formatBriefArgs + suggestions 辅助方法
│   ├── phase.go                #   处理阶段常量：Ready/Thinking/Reasoning/Running + setPhase
│   ├── view.go                 #   View 组装：JoinVertical 堆叠全部 section
│   ├── block_stream.go         #   BlockStream：线程安全 ContentBlock 缓冲区 + 逐字释放
│   └── components/             #   UI 组件
│       ├── header.go           #     Header：猫脸 + 应用名 + token 用量 + provider
│       ├── input.go            #     Input：textarea + 发送历史 + 发送过渡态
│       ├── messages.go         #     Messages：滚动视口 + auto-follow + 处理状态
│       ├── message.go          #     ChatMessage → 按 Role 分发 MessageItem
│       ├── message_items.go    #     User/Assistant/System/Error 消息项 + thickLeftBar + stripLeadingSpaces
│       ├── processing.go       #     ProcessingItem：spinner + 状态 + 流式块 + token 计数
│       ├── splash.go           #     Splash：ASCII 猫 + 猫眼闪烁
│       ├── list_widget.go      #     List：通用虚拟滚动列表 + 鼠标滚轮 + Height 估算
│       ├── content_block.go    #     ContentBlock：工具卡片 + 💭 思考块 + 文本块
│       ├── scrollbar.go        #     Scrollbar：独立滚动指示器组件
│       ├── suggestions.go      #     Suggestions：斜杠命令自动补全
│       └── confirm_bar.go      #     ConfirmBar：工具确认卡片
│   └── styles/                 #   样式
│       ├── colors.go           #     色彩体系 + Styles 结构体
│       ├── charset.go          #     制表符字符集（含 ASCII 回退）
│       └── markdown.go         #     glamour 封装（tokyo-night 主题 + 全宽度预热 + zero margin）
├── docs/                       # 文档
├── go.mod / go.sum
└── README.md
```

## 包依赖图

```
main
  ├── bot ──────┬── bot/types ─── bot/tools
  │             ├── bot/extensions ─── bot/tools
  │             ├── bot/agent ───┬── bot/tools
  │             │                ├── bot/types
  │             │                ├── bot/ctxmgr ─── llm
  │             │                └── llm
  │             └── ctxmgr
  └── tui ──────┬── bot (BotInterface)
                └── components ──┬── bot/types (confirm_bar.go)
                                 │── bot/tools (content_block.go)
                                 └── styles (stdlib + lipgloss + glamour)
```

## 核心架构：Agent 循环

```
用户输入
  │
  ▼
┌──────────────────────────────────────────────┐
│  Run() 主循环（最多 15 轮）                    │
│                                              │
│  state = stepState{input}                    │
│                                              │
│  ┌─ for !finished && step < maxIterations ─┐ │
│  │                                          │ │
│  │  ① Reason(state) → ReasoningResult      │ │
│  │     ├─ / 命令 → ActionFinish             │ │
│  │     └─ callLLMForTool()                  │ │
│  │         ├─ phaseFn("Thinking")           │ │
│  │         ├─ ctxMgr.Build(true) 组装上下文  │ │
│  │         ├─ llmClient.ChatStream() 流式   │ │
│  │         ├─ withRetry() 指数退避重试       │ │
│  │         ├─ 首 token → phaseFn("Reasoning")│ │
│  │         ├─ 返回 tool_calls → 解析        │ │
│  │         ├─ 保存 ReasoningContent         │ │
│  │         └─ 无 tool_calls → 直接文本回复   │ │
│  │                                          │ │
│  │  ② ExecuteBatch(calls) → results         │ │
│  │     ├─ Surface TextContent 为 "think"    │ │
│  │     ├─ needsSequential? → 串行/并行      │ │
│  │     ├─ 检查 DangerLevel                  │ │
│  │     ├─ LevelWrite+ → confirmFn 确认       │ │
│  │     └─ tool.Execute() 执行工具            │ │
│  │                                          │ │
│  │  ③ Feedback(state, result)              │ │
│  │     ├─ step++ / shouldStop / shouldRetry │ │
│  │     └─ 构建下一步 stepState              │ │
│  │                                          │ │
│  │  ShouldStop? → forceSynthesize()         │ │
│  └──────────────────────────────────────────┘ │
│                                              │
│  返回 RunResult{FinalOutput, Steps}          │
└──────────────────────────────────────────────┘
```

### stepState 状态传递

```go
type stepState struct {
    Input          string  // 用户原始输入
    PreviousAction string  // 上一轮动作
    PreviousOutput string  // 上一轮输出
    Success        bool    // 上一轮是否成功
    RetryCount     int     // 重试次数
    SearchCount    int     // web_search 累计调用数
    FetchCount     int     // web_fetch 累计调用数
}
```

### Stop 策略

- `shouldStop`: 自定义停止条件（SearchCount >= 4 && FetchCount == 0 → 强制综合）
- `contextTransform`: 工具结果 > 6 条时注入 "现在综合回答" 指令
- `maxIterations`: 15 轮后触发 `forceSynthesize()`

### BTW 中断与 Steer 机制

Agent 处理中用户可输入新消息打断当前 LLM 调用并注入到上下文：

```
┌─── TUI 层 ──┐     ┌─── Agent 层 ──┐
│ Enter 按键   │     │                │
│ m.Bot.Steer()│────→│ steeringCh ← msg
│ Stream.Reset │     │ replaceCtx()   │  ← 原子取消旧 ctx + 创建新 ctx
│ SetBlocks(nil)│    │                │
│ status 更新   │     │ drainSteering()│  ← 拾取消息注入 ctxMgr
└──────────────┘     │ continue       │  ← 新 Reason() 处理
                     └────────────────┘
```

- **Enter** → `Steer()`：注入消息 + 中断当前 LLM 调用，UI 重置 processingStart + Stream
- **Esc** → `Abort()`：设置 `finished=true` + 取消 ctx，退出循环返回 "已中断"
- `callLLMForTool` 检测 `context.Canceled` → 返回 `Interrupted` 标记
- Run 循环检查 `Interrupted`：`finished=true` → Abort；`finished=false` → drain + continue
- UI 反馈：Stream 清空 + processingStart 重置 + status 显示 "Processing new input..."

### 指数退避重试

`retry.go`：LLM 调用失败时自动重试，指数退避 0.5s→1s→2s→4s→8s（最多 4 次）。

```
callLLMForTool() / forceSynthesize()
  └─ withRetry(ctx, fn) ──┐
     ├─ fn() 执行           │
     ├─ 成功 → 返回         │
     └─ 失败 → isRetryable?  │
        ├─ 可重试 (5xx/429/network/timeout) → 退避后重试
        └─ 不可重试 (4xx/cancel/deadline) → 立即返回错误
```

- `callLLMForTool` 的整个 ChatStream + token 处理包裹在 retry 闭包内
- `forceSynthesize` 同样包裹
- `context.Canceled` 和 `context.DeadlineExceeded` 不重试（用户取消了就是取消了）

## 共享类型与接口

### BotInterface (tui/model.go)

```go
type BotInterface interface {
    RunAgent(input string, onStep func(step int, thought, action, toolName, toolArgs, output string, batchIdx, batchTotal int)) (string, error)
    ExecuteCommand(input string) (string, bool)
    TokenUsage() (prompt, completion int)
    ContextTokens() int
    Duration() string
    CommandNames() []string
    SetConfirmFn(types.ConfirmFunc)
    SetPhaseFn(types.PhaseFunc)
    Steer(msg string)
    Abort()
    SetStreamFn(fn func(delta string))
    Provider() string
    Model() string
}
```

### ConfirmRequest (bot/types)

```go
type ConfirmRequest struct {
    ToolName string
    Args     map[string]interface{}
    Level    tools.DangerLevel
    Response chan bool          // TUI 通过此 channel 返回确认结果
}
```

## LLM 抽象层

### 接口

```go
type LLM interface {
    Chat(ctx, messages []Message, tools []ToolDef) (*Response, error)
    ChatStream(ctx, messages []Message, tools []ToolDef) (<-chan StreamToken, <-chan error)
    SetAPIKey(apiKey string)
    SetBaseURL(url string)
}
```

### 共享 HTTP 客户端

`llm.go` 导出 `SharedHTTPClient`（无超时，用于流式）和 `SharedHTTPClientTimeout`（120s 超时，用于同步调用）。底层共享 `http.Transport` 连接池（MaxIdleConns=20, IdleConnTimeout=90s）。

### Provider 适配

| Provider | 实现文件 | 特殊处理 |
|----------|---------|---------|
| OpenAI / GLM / DeepSeek | `openai_compat.go` | `/chat/completions`，流式 tool_calls 遍历 |
| Anthropic | `anthropic.go` | `/v1/messages`，SSE content_block_start/delta 解析 |

## 工具系统

### Tool 接口

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() []Parameter
    ExecutionMode(args map[string]interface{}) ExecutionMode  // ModeParallel / ModeSequential
    DangerLevel(args map[string]interface{}) DangerLevel
    Execute(ctx context.Context, args map[string]interface{}) (string, error)
}
```

### 执行模式

| 模式 | 说明 | 工具 |
|------|------|------|
| `ModeParallel` | 可并发执行 | glob, grep, web_search, web_fetch, filesystem(read/list) |
| `ModeSequential` | 独占执行 | bash, edit, filesystem(write) |

### 危险等级

```
LevelSafe        (0) — 只读，自动放行
LevelWrite       (1) — 写操作，弹框确认
LevelDestructive (2) — 破坏操作，弹框确认
LevelForbidden   (3) — 永远禁止
```

### 内建工具

| 工具 | 文件 | 说明 |
|------|------|------|
| BashTool | `tool_bash.go` | Shell 命令执行，四级关键词匹配分级，ANSI 转义序列清理 |
| FileSystemTool | `tool_filesystem.go` | 文件 read/write/list，SHA256 读缓存去重，智能截断 |
| GlobTool | `tool_glob.go` | 文件模式匹配 |
| GrepTool | `tool_grep.go` | ripgrep 内容搜索，支持 regex/glob/context |
| EditTool | `tool_edit.go` | 精确字符串替换，失败返回带行号的文件内容 + diff |
| WebSearchTool | `tool_websearch.go` | Exa AI MCP 优先（free tier），Bing HTML 降级，numResults 可配 |
| WebFetchTool | `tool_webfetch.go` | HTTP GET + HTML→Markdown，DNS 内网校验，prompt 指导提取 |

## 确认机制

```
Agent goroutine                    TUI goroutine
  │                                  │
  ├─ executeOne()                    │
  ├─ level >= LevelWrite             │
  ├─ confirmFn(req) ────→ confirmCh  │
  │  (阻塞)               ↓          │
  │                    listenConfirm │
  │                       ↓          │
  │                    confirmMsg    │
  │                       ↓          │
  │                  ConfirmBar.View │
  │                  [enter]/[esc]   │
  │  ← req.Response ←───┘            │
  ├─ continue / deny                 │
```

## TUI 组件树

```
Model
├── Header         — (=^.^=) PRIMUS v0.1.0 · tokens · provider/model
├── Splash         — 启动页 (ASCII 猫 + 猫眼闪烁)
├── Messages + Scrollbar — 消息列表 + 独立滚动指示器
│   ├── UserMessageItem        — 金色 ▐ 粗条 "You"
│   ├── AssistantMessageItem   — 青色 ▐ 粗条 "Assistant" + ContentBlock + Footer
│   ├── SystemMessageItem      — 蓝色 ▐ 粗条 "·"
│   ├── ErrorMessageItem       — 红色 ▐ 粗条 "!"
│   └── ProcessingItem         — 绿色 │ ◉ spinner + Phase + Blocks
├── Suggestions    — 斜杠命令自动补全
├── Input          — 消息输入框（历史翻阅、tab 补全）
└── ConfirmBar     — 工具确认卡片
```

### 视觉方案（Plan B：厚左色条）

每条消息用 `▐`（U+2590）厚色条 + `PaddingLeft(1)` 替代旧版 `│` 细线。lipgloss `BorderLeft` 块级渲染，不与内容 ANSI 混拼。

| 角色 | 色条颜色 |
|------|---------|
| User | 暖金 `#c9a96e` |
| Assistant | 青色 `#4ec9b0` |
| System | 蓝色 `#7a8ba0` |
| Error | 红色 `#e06c75` |

### 结构化内容块 (ContentBlock)

```go
type ContentBlock struct {
    Type       BlockType  // BlockToolCall / BlockThinking / BlockText
    Content    string     // 渲染文本
    ToolName   string     // 工具名（仅 BlockToolCall）
    ToolArgs   string     // 简要参数
    Collapsed  bool       // 折叠状态
    BatchIdx   int        // 并行批次序号
    BatchTotal int        // 并行批次总数
}
```

- **BlockToolCall**: 聚合在单一 `NormalBorder` 卡片中，暖金色边框。折叠态显示 `◆ toolName args [+]`，展开态显示完整输出。多个工具共用一个卡片
- **BlockThinking**: `💭` 前缀 + Subtle 色，多行自动缩进对齐，工具卡前后自动加空行
- `ctrl+e` 翻转所有 tool block 的 Collapsed

### 处理阶段 (phase.go)

```
Ready ──→ Thinking ──→ Reasoning ──→ Running ──→ Thinking ──→ ... ──→ Ready
```

| 阶段 | 触发 |
|------|------|
| Ready | 空闲 |
| Thinking | transitionTo(StateProcessing) / Reason() |
| Reasoning | callLLMForTool 首 token |
| Running | executor 开始执行工具 |

### 流式输出

不展示 raw markdown token。处理中只显示结构化块（💭 思考 + 工具卡片），最终回答一次 glamour 渲染。

### TUI 消息流

```go
tea.Batch(
    m.Spinner.Tick,           // spinner 动画
    listenConfirm(confirmCh), // 确认请求监听
    func() tea.Msg {          // Agent 执行 (goroutine)
        m.Bot.RunAgent(input, callback)
        return doneMsg{content, duration, tokens, err}
    },
)
```

- `think` 回调 → `BlockThinking`追加到 Stream
- `execute_tool` 回调 → `BlockToolCall`追加到 Stream
- `chat` 回调 → 设置 `finalResponse` + BlockThinking
- `handleDone` 滤掉最后一条 BlockThinking（最终回答），其余保留在 Blocks

### Footer 元数据

`doneMsg` 携带 `duration` 和 `tokens`，通过 `ChatMessage.Footer` 字段传递。渲染为 Subtle 色文本，置于消息内容下方，不经 glamour。

## 上下文管理

`bot/ctxmgr/` 拆分为 4 文件：

- `manager.go`: Manager struct + Build() 组装 + 统计
- `storage.go`: Add/AddToolResult/Clear
- `token.go`: 语言感知 token 估算（CJK ~1.5/token, ASCII ~4/token, ceiling 除法防零）
- `summarize.go`: Summarize/BuildPrompt — **结构化摘要**

核心策略：**Hybrid Window + Structured Summary**。窗口大小 20，token 预算默认 64000。超出时 `Summarize()` 将前半压缩为六段结构化摘要：

```
[目标] 用户正在完成的目标
[进展] 已完成 / 进行中 / 阻塞
[关键决策] 技术选型、架构决策
[下一步] 待执行的操作
[关键上下文] 用户偏好、约束条件
[相关文件] 关键文件路径及作用
```

- 增量更新：已有摘要时在原基础上增删改，而非重新生成
- 锚定策略：连续多次压缩保持信息连续性
- Tool 消息保留 800 字符截断（信息密度更高），其他 500
- BuildPrompt 英文模板，compaction agent 无工具纯文本

## Markdown 渲染

`glamour` 封装（tokyo-night 主题 + document margin=0 + 全宽度预热 40-160）。

- `Warmup()` 启动时预创建 121 个 renderer，避免懒加载 chroma lexer（~6s）阻塞首次渲染
- `PaddingLeft(1)` 统一提供消息缩进，glamour margin=0 保证一致性
- `stripLeadingSpaces()` 兼容旧 glamour 输出

## 配置

`~/.primusbot/config.json`：

```json
{
  "provider": "openai",
  "api_key": "sk-...",
  "model": "gpt-4",
  "base_url": "https://api.openai.com/v1",
  "token_budget": 128000
}
```

支持 provider: `openai` / `anthropic` / `glm`。
