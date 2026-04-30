# PrimusBot 架构文档

## 项目概述

PrimusBot 是一个基于 Go 的终端 AI 助手，支持多 LLM provider（OpenAI / Anthropic / GLM），具备 Agent 循环、工具调用（Function Calling）和权限确认机制。

## 目录结构

```
primusbot/
├── main.go                  # 入口：交互模式(TUI) 或 非交互模式(CLI)
├── bot/                     # 核心逻辑包（组装 + 命令 + 配置）
│   ├── bot.go               #   Bot 构造函数、依赖组装、公开 API
│   ├── command.go           #   命令系统：斜杠命令解析与注册
│   ├── config.go            #   配置管理 (~/.primusbot/config.json)
│   ├── prompt/
│   │   └── system.md        #   角色 system prompt
│   ├── agent/               #   Agent 循环子包
│   │   ├── agent.go          #     Agent 结构体、构造函数、Reset
│   │   ├── run.go            #     主循环：Reason → Execute → Feedback
│   │   ├── reasoner.go       #     决策：LLM 推理 + Native Function Calling + ToolDef 转换
│   │   ├── executor.go       #     执行：工具调度 + DangerLevel 检查 + 确认回调
│   │   ├── feedback.go       #     反馈状态机
│   │   ├── memory.go         #     步骤记忆
│   │   └── confirm.go        #     确认机制：ConfirmRequest / ConfirmFunc
│   └── tools/               #   工具系统子包
│       ├── tool.go           #     Tool 接口、Registry、Descriptor、DangerLevel、ParseCall
│       ├── tool_bash.go      #     BashTool — Shell 命令执行 + 危险分级
│       ├── tool_filesystem.go #    FileSystemTool — 文件读写列表
│       ├── tool_glob.go      #     GlobTool — 文件模式匹配
│       └── default.go        #     内建工具注册
├── ctxmgr/                  # 上下文管理包
│   └── manager.go            #   Manager 消息存储 + Build 上下文组装
├── llm/                     # LLM 客户端包（零项目依赖）
│   ├── llm.go               #   LLM 接口、Message/Response/ToolDef 类型
│   ├── openai.go            #   OpenAI 兼容实现（含 GLM）
│   ├── anthropic.go         #   Anthropic 实现（含 tool_use 双向转换）
│   ├── glm.go               #   GLM 智谱实现
│   └── event_reader.go      #   SSE 流解析器
├── tui/                     # 终端 UI 包（Bubble Tea v2，共享 Model 单包）
│   ├── tui.go               #   程序入口：创建 Program，运行主循环
│   ├── model.go             #   Model 定义、初始化、listenConfirm goroutine
│   ├── update.go            #   Update 消息循环：按键分流、确认、历史翻阅、命令提示
│   ├── view.go              #   View 渲染：Splash/Header/Messages/Suggestions/ConfirmBar/Input
│   ├── commands.go           #   用户交互调度：startChat、startAgent、命令提示逻辑
│   ├── stream.go            #   流式输出状态管理：线程安全累积、快照、变更检测
│   ├── components/          #   UI 组件子包
│   │   ├── header.go         #     Header：猫脸 + 应用名 + token 用量 + provider/model
│   │   ├── input.go          #     Input：textarea + 历史翻阅状态机 + 发送过渡态
│   │   ├── messages.go       #     Messages：滚动视口、auto-follow、流式文本显示
│   │   ├── message.go        #     ChatMessage 模型 + 按 Role 分发 MessageItem
│   │   ├── message_items.go  #     User/Assistant/System/Error 消息项渲染（宽度缓存）
│   │   ├── processing.go     #     Processing：◉ spinner + 流式文本实时渲染
│   │   ├── splash.go         #     Splash：ASCII 猫 + 猫眼闪烁动画
│   │   ├── list_widget.go    #     ListWidget：通用滚动列表 + 鼠标滚轮
│   │   └── util.go           #     工具函数（maxInt 等）
│   └── styles/              #   样式定义子包
│       ├── colors.go         #     色彩体系：深夜书房主题 + Styles/DefaultStyles
│       ├── charset.go        #     box-drawing 字符集常量
│       └── markdown.go       #     Markdown 渲染器：标题/列表/代码块/粗斜体
├── test/                    # 测试
│   └── bot_test.go
├── docs/                    # 文档
│   ├── ARCHITECTURE.md       #   本文档
│   └── DESIGN.md             #   设计文档
├── go.mod
├── go.sum
├── config.example.json
└── README.md
```

## 包依赖图

```
main.go
  ├── bot
  │     ├── tools   (独立，仅依赖 stdlib)
  │     ├── ctxmgr  (独立，仅依赖 llm)
  │     └── llm     (独立，仅依赖 stdlib)
  └── tui
        └── bot
```

`tools`、`ctxmgr`、`llm` 三者彼此无依赖。`bot` 是唯一的胶水层，组装所有模块。

## 核心架构：Agent 循环

```
用户输入
  │
  ▼
┌──────────────────────────────────────────────┐
│  Run() 主循环                                 │
│                                              │
│  state = stepState{input}                    │
│                                              │
│  ┌─ for !finished && step < maxIterations ─┐ │
│  │                                          │ │
│  │  ① Reason(state) → ReasoningResult      │ │
│  │     ├─ / 命令 → ActionFinish             │ │
│  │     ├─ 上一轮工具成功 → callLLMForResponse│ │
│  │     └─ 否则 → callLLMForTool             │ │
│  │              ├─ Native Function Calling  │ │
│  │              ├─ LLM 返回 tool_calls      │ │
│  │              └─ LLM 返回 text → 直接回复  │ │
│  │                                          │ │
│  │  ② Execute(reasoning) → ActionResult     │ │
│  │     ├─ ActionChat → 返回文本             │ │
│  │     └─ ActionExecuteTool → executeTool   │ │
│  │           ├─ 检查 DangerLevel            │ │
│  │           ├─ LevelForbidden → 拒绝       │ │
│  │           ├─ LevelWrite/Destructive      │ │
│  │           │    → confirmFn → TUI 确认框   │ │
│  │           └─ 执行 tool.Execute()         │ │
│  │                                          │ │
│  │  ③ Feedback(state, result)              │ │
│  │     → 更新 Memory                        │ │
│  │     → 构建下一步 stepState               │ │
│  │     → 返回 (newState, retry, stop)       │ │
│  │                                          │ │
│  └──────────────────────────────────────────┘ │
│                                              │
│  返回 RunResult{FinalOutput, Steps}          │
└──────────────────────────────────────────────┘
```

### stepState 状态传递

```go
type stepState struct {
    input          string  // 用户原始输入
    previousAction string  // 上一轮动作 (execute_tool / chat)
    previousOutput string  // 上一轮输出 (工具结果)
    success        bool    // 上一轮是否成功
    retryCount     int     // 重试次数
}
```

状态在 Feedback → Reason 之间传递，驱动多轮工具调用。

## LLM 抽象层

### 接口定义

```go
type LLM interface {
    Chat(ctx, messages []Message, tools []ToolDef) (*Response, error)
    ChatStream(ctx, messages []Message, tools []ToolDef) (<-chan StreamToken, <-chan error)
    SetAPIKey(apiKey string)
    SetBaseURL(url string)
}
```

### 核心类型

| 类型 | 用途 |
|------|------|
| `Message` | 统一消息格式（含 ToolCalls / ToolCallID） |
| `ToolCall` | 结构化函数调用（ID + Function.Name + Function.Arguments） |
| `ToolDef` | 工具定义（FunctionDef + Parameters + Properties） |
| `Response` | LLM 响应（Choices + Usage） |
| `StreamChunk` | 流式 SSE 数据块 |

### Provider 适配

| Provider | 实现 | 特殊处理 |
|----------|------|----------|
| OpenAI | `openai.go` | 原生支持 tools + tool_choice |
| Anthropic | `anthropic.go` | 双向转换：ToolDef↔anthropic_tool, ContentBlock↔ToolCall, Message↔anthropicMsg；system prompt 通过独立字段传递 |
| GLM 智谱 | `glm.go` | 兼容 OpenAI 格式，额外处理 reasoning_content |

`LastToolCalls(resp)` 辅助函数从 Response 中提取 tool_calls。

## 工具系统

### Tool 接口

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() []Parameter
    DangerLevel(args map[string]interface{}) DangerLevel
    Execute(ctx context.Context, args map[string]interface{}) (string, error)
}
```

### 危险等级

```
LevelSafe         (0) — 只读操作，自动放行（ls, cat, find, read, list, glob）
LevelWrite        (1) — 写操作，弹框确认（mkdir, cp, write, git commit）
LevelDestructive  (2) — 破坏操作，弹框确认（rm, chmod, kill, git push --force）
LevelForbidden    (3) — 永远禁止（sudo, eval, curl|bash, ssh）
```

### 内建工具

| 工具 | 文件 | 安全等级判断 |
|------|------|-------------|
| BashTool | `tool_bash.go` | 命令关键词匹配：forbidden patterns → LevelForbidden；destructive patterns → LevelDestructive；write patterns → LevelWrite；其余 → LevelSafe |
| FileSystemTool | `tool_filesystem.go` | write → LevelWrite；read/list → LevelSafe |
| GlobTool | `tool_glob.go` | 始终 LevelSafe |

### 工具描述符转换

`descriptorsToToolDefs()` 在 `reasoner.go` 中将 `[]tools.Descriptor` 转为 `[]llm.ToolDef`，供 Native Function Calling 使用。

### 工具调用协议

`tools.ParseCall(input) → (name, args, error)` 解析 `"toolName:key1=val1,key2=val2"` 格式，将工具调用协议封装在 tools 包内。

## 确认机制

```
Agent goroutine                    TUI goroutine
  │                                  │
  ├─ executeTool()                   │
  ├─ tool.DangerLevel(args)          │
  ├─ level >= Write?                 │
  ├─ confirmFn(req) ────→ confirmCh  │
  │  (阻塞)               ↓          │
  │                    listenConfirm │
  │                       ↓          │
  │                    confirmMsg    │
  │                       ↓          │
  │                  renderConfirmBar│
  │                  [enter]/[esc]   │
  │  ← req.Response ←───┘            │
  ├─ continue / deny                 │
```

`Bot.SetConfirmFn()` 设置确认回调。Agent 调用 `confirmFn` 时阻塞，TUI 通过 `listenConfirm` goroutine 接收请求，渲染确认栏，用户按键后通过 `req.Response` channel 返回结果。

## TUI 组件树

```
Model
├── Header         — (=^.^=) PRIMUS v0.1.0 · 1.2k/8k · provider/model
├── Splash         — 启动页 (ASCII 猫 + 猫眼闪烁)
├── Messages       — 消息列表容器
│   ├── UserMessageItem        — 金色竖线 "You" + 内容
│   ├── AssistantMessageItem   — 绿色竖线 "Assistant" + 推理(dim) + 内容
│   ├── SystemMessageItem      — 蓝色竖线 "·" + 内容
│   ├── ErrorMessageItem       — 红色竖线 "!" + 内容
│   └── ProcessingItem         — 绿色竖线 "◉ Thinking" + 流式文本
├── Suggestions    — 命令提示 popup（输入 / 时自动弹出）
├── Input          — 消息输入框（含 history 翻阅、tab 补全）
└── ConfirmBar     — 工具确认栏 [level] tool args [enter]/[esc]
```

## TUI 消息流

```
tea.Batch(
    m.Spinner.Tick,           // spinner 动画
    listenConfirm(confirmCh), // 确认请求监听
    func() tea.Msg {          // Agent 执行
        m.Bot.RunAgent(input, callback)
        return doneMsg{content, reasoning, err}
    },
)
```

- `callback` 将工具步骤写入 `StreamState`，区分 tool 步（进度文本）和 chat 步（最终回复）
- `handleSpinnerTick` 读取 StreamState 更新流式显示
- `handleDone` 将最终回复作为 AssistantMessage 添加到消息列表，并调用 `updateTokens()` 刷新 Header token 显示
- `handleConfirmKey` 处理确认框的 enter/esc
- `refreshSuggestions` 在每次按键时更新 `/` 命令提示，`acceptSuggestion` 填入选中项

### Token 实时显示

Header 栏每次消息发送和回复完成后更新 token 用量：

```
(=^.^=) PRIMUS v0.1.0 · 1.2k/8k · openai/gpt-4
                        ───────
                        灰色(<60%) / 黄色(60-90%) / 红色(≥90%)
```

`Bot.TokenUsage()` → `ctxMgr.TokenUsage()` 返回 (estimatedTokens, budget)。

### 命令提示

输入 `/` 时自动弹出可用命令列表：

```
── suggestions ──
> /help            ← teal 高亮
  /clear
  /summarize
  /stats
  /config
```

Tab / Shift+Tab 选择，Enter 填入（光标移到末尾可继续输入参数），再按 Enter 发送。输入精确匹配时（如 `/help`）不弹框直接可用。

## 上下文管理

`ctxmgr.Manager` 实现 **Hybrid Window + Summary** 方案：

### 存储结构

```
Manager
├── systemPrompt   — 角色 prompt（始终保留，不参与截断）
├── summary        — 旧对话压缩摘要（由 LLM 生成）
├── messages[]     — 滑动窗口内的最近消息（最多 windowSize=20 条）
├── windowSize     — 滑动窗口大小
├── tokenBudget    — Build() 输出的 token 上限（默认 8000）
└── summarizer     — LLM 摘要回调（由 bot 注入）
```

### Build() 上下文组装

```
┌─────────────────────────────┐
│ system: 角色 prompt          │  ← 始终保留
├─────────────────────────────┤
│ system: [历史摘要]           │  ← 旧消息压缩（仅当触发过摘要）
├─────────────────────────────┤
│ user: 最近消息 (≤20条)       │  ← sliding window
│ assistant: ...              │
├─────────────────────────────┤
│ system: 工具调用指令          │  ← Build(withTools=true) 时附加
└─────────────────────────────┘
```

Token 超预算时从最早的消息对开始丢弃，保证不超出模型上限。

### 自动摘要

```
触发条件: 消息 > 20 条 且 token > budget/2
执行时机: 每次 RunAgent() 完成后调用 SummarizeIfNeeded()
摘要策略: 将最早 50% 消息传给 LLM → 生成 ≤300 字摘要 → 从消息列表移出
容错:     摘要 LLM 调用失败时恢复原始消息，不丢数据
```

### API

```go
ctxMgr.Add("user", content)          // 存消息
ctxMgr.Build(false)                  // 组装聊天上下文（含角色 prompt）
ctxMgr.Build(true)                   // 组装工具上下文（附加工具指令）
ctxMgr.SetSummarizer(fn)             // 设置摘要回调
ctxMgr.NeedsSummarization() → bool   // 是否需要摘要
ctxMgr.Summarize()                   // 执行摘要
ctxMgr.Clear()                       // 清空消息+摘要
ctxMgr.Stats() → (count, tokens, hasSummary)
ctxMgr.TokenUsage() → (tokens, budget)
```

## 数据流

```
用户输入 "帮我列出当前目录"
  │
  ▼
startChat() → ExecuteCommand() → (非命令) → startAgent()
  │
  ▼
RunAgent(input, callback)
  │
  ▼
Agent.Run()
  ├─ ctxMgr.Add("user", input)
  ├─ Reason(stepState{input})
  │   └─ callLLMForTool()
  │       ├─ ctxMgr.Build(true) → [system: persona, ...history, system: tool指令]
  │       ├─ + descriptorsToToolDefs()
  │       └─ llmClient.Chat(messages, toolDefs)
  │           └─ LLM 返回 tool_calls: [{name:"bash", arguments:"{\"command\":\"ls\"}"}]
  ├─ Execute()
  │   └─ executeTool("bash:command=ls")
  │       ├─ DangerLevel(args) → LevelSafe → 自动放行
  │       └─ BashTool.Execute() → 目录列表文本
  ├─ Feedback() → stepState{previousAction:"execute_tool", success:true}
  ├─ Reason(stepState{success})
  │   └─ callLLMForResponse()
  │       ├─ ctxMgr.Build(false) → [system: persona, ...history]
  │       └─ llmClient.Chat(messages, nil)
  │           └─ LLM 返回: "主人～当前目录有这些文件喵！..."
  ├─ Execute() → ActionChat → 最终回复
  ├─ ctxMgr.Add("assistant", finalOutput)
  └─ Feedback() → shouldStop=true
  │
  ▼
RunAgent 返回 → SummarizeIfNeeded() → 消息过多时触发摘要
  │
  ▼
doneMsg{content: "主人～当前目录...", reasoningContent: "> 调用工具: bash `bash(command=ls)`"}
  │
  ▼
handleDone() → Messages.AddMessage(AssistantMessage)
```

## 配置

配置文件路径：`~/.primusbot/config.json`

```json
{
  "provider": "openai",
  "api_key": "sk-...",
  "model": "gpt-4",
  "base_url": "https://api.openai.com/v1"
}
```

支持 provider: `openai` / `anthropic` / `glm`。
