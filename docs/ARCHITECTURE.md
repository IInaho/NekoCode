# PrimusBot 架构文档

## 项目概述

PrimusBot 是一个基于 Go 的终端 AI 助手，使用 Bubble Tea v2 构建 TUI，支持多 LLM provider（OpenAI / Anthropic / GLM），具备 Agent 循环、Native Function Calling、工具执行和权限确认机制。

## 目录结构

```
primusbot/
├── main.go                     # 入口：无参→TUI 交互模式，有参→单次 CLI 模式
├── bot/                        # 核心组装层
│   ├── bot.go                  #   Bot 结构体、依赖注入、公开 API
│   ├── command.go              #   斜杠命令系统（/help /clear /stats /summarize /config）
│   ├── config.go               #   配置管理 (~/.primusbot/config.json)
│   ├── prompt/system.md        #   [embed] 角色 system prompt
│   ├── types/types.go          #   共享类型：ConfirmRequest / ConfirmFunc / PhaseFunc
│   ├── agent/                  #   Agent 循环
│   │   ├── agent.go            #     Agent 结构体、构造函数、Reset
│   │   ├── run.go              #     主循环 (Reason→Execute→Feedback) + 状态机
│   │   ├── reasoner.go         #     LLM 调用、工具调用解析、ToolDef 转换
│   │   └── executor.go         #     工具调度、DangerLevel 检查、确认回调
│   └── tools/                  #   工具系统
│       ├── tool.go             #     Tool 接口、Registry、ParseCall、SplitPairs
│       ├── tool_bash.go        #     BashTool — Shell 命令执行 + 三级危险分级
│       ├── tool_filesystem.go  #     FileSystemTool — 文件读写列表
│       ├── tool_glob.go        #     GlobTool — 文件模式匹配
│       ├── tool_grep.go        #     GrepTool — ripgrep 内容搜索
│       └── tool_edit.go        #     EditTool — 精确字符串替换
├── ctxmgr/                     # 上下文管理
│   └── manager.go              #   Manager：消息存储 + Build 上下文组装 + 摘要
├── llm/                        # LLM 抽象层（零项目内部依赖）
│   ├── llm.go                  #   LLM 接口、Message/Response/ToolDef 等核心类型
│   ├── openai_compat.go        #   OpenAI 兼容实现（OpenAI / GLM / DeepSeek）
│   ├── anthropic.go            #   Anthropic 实现（含 tool_use/tool_result 双向转换）
│   └── event_reader.go         #   SSE 流解析器
├── tui/                        # 终端 UI（Bubble Tea v2）
│   ├── tui.go                  #   Run() 入口：创建 Program，启动事件循环
│   ├── model.go                #   Model 定义、BotInterface、状态机、初始化
│   ├── update.go               #   消息路由：窗口调整、spinner、按键、确认、done
│   ├── view.go                 #   视图组合：Splash/Header/Messages/ConfirmBar/Input
│   ├── commands.go             #   startChat/startAgent：用户输入调度 + agent 执行
│   ├── block_stream.go         #   BlockStream：线程安全 ContentBlock 缓冲区
│   └── components/             #   UI 组件
│       ├── header.go           #     Header：猫脸 + 应用名 + token 用量 + provider
│       ├── input.go            #     Input：textarea + 发送历史 + 发送过渡态
│       ├── messages.go         #     Messages：滚动视口 + auto-follow + 流式块
│       ├── message.go          #     ChatMessage → 按 Role 分发 MessageItem
│       ├── message_items.go    #     User/Assistant/System/Error 消息项渲染
│       ├── processing.go       #     ProcessingItem：spinner + 状态文本 + 流式块
│       ├── splash.go           #     Splash：ASCII 猫 + 猫眼闪烁
│       ├── list_widget.go      #     List：通用虚拟滚动列表 + 鼠标滚轮
│       ├── content_block.go    #     ContentBlock：工具调用/思考/文本 结构化块
│       ├── suggestions.go      #     Suggestions：斜杠命令自动补全
│       └── confirm_bar.go      #     ConfirmBar：工具确认卡片
│   └── styles/                 #   样式
│       ├── colors.go           #     色彩体系 + Styles 结构体
│       ├── charset.go          #     制表符字符集（含 ASCII 回退）
│       └── markdown.go         #     Markdown 渲染器 + diff 高亮
├── docs/                       # 文档
├── go.mod / go.sum
└── README.md
```

## 包依赖图

```
main
  ├── bot ──────┬── bot/types ─── bot/tools (stdlib only)
  │             ├── bot/agent ───┬── bot/tools
  │             │                ├── bot/types
  │             │                ├── ctxmgr ─── llm (stdlib only)
  │             │                └── llm
  │             └── ctxmgr
  └── tui ──────┬── bot (BotInterface)
                └── components ──┬── bot/types (仅 confirm_bar.go)
                                 └── styles (stdlib + lipgloss)
```

`bot/tools` 和 `llm` 是叶子包，不依赖项目内任何包。`bot` 是唯一胶水层。`tui` 仅依赖 `bot`（通过 `BotInterface` 接口），`tui/components` 仅依赖 `bot/types` 共享类型。

## 核心架构：Agent 循环

```
用户输入
  │
  ▼
┌──────────────────────────────────────────────┐
│  Run() 主循环（最多 10 轮）                    │
│                                              │
│  state = stepState{input}                    │
│                                              │
│  ┌─ for !finished && step < maxIterations ─┐ │
│  │                                          │ │
│  │  ① Reason(state) → ReasoningResult      │ │
│  │     ├─ / 命令 → ActionFinish             │ │
│  │     └─ callLLMForTool()                  │ │
│  │         ├─ ctxMgr.Build(true) 组装上下文  │ │
│  │         ├─ llmClient.Chat() 调用 LLM     │ │
│  │         ├─ 返回 tool_calls → 解析首条    │ │
│  │         └─ 无 tool_calls → 直接文本回复   │ │
│  │                                          │ │
│  │  ② Execute(reasoning) → ActionResult     │ │
│  │     ├─ ActionChat → 返回文本 (IsFinal)    │ │
│  │     ├─ ActionFinish → 完成               │ │
│  │     └─ ActionExecuteTool → executeTool   │ │
│  │         ├─ ParseCall 解析工具调用         │ │
│  │         ├─ 检查 DangerLevel              │ │
│  │         ├─ LevelWrite+ → confirmFn 确认   │ │
│  │         └─ tool.Execute() 执行工具        │ │
│  │                                          │ │
│  │  ③ Feedback(state, result)              │ │
│  │     ├─ step++ / shouldStop / shouldRetry │ │
│  │     └─ 构建下一步 stepState              │ │
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
    previousAction string  // 上一轮动作
    previousOutput string  // 上一轮输出
    success        bool    // 上一轮是否成功
    retryCount     int     // 重试次数
}
```

## 共享类型 (bot/types)

TUI 和 Agent 之间的解耦通过 `bot/types` 包实现：

```go
type ConfirmRequest struct {
    ToolName string
    Args     map[string]interface{}
    Level    tools.DangerLevel
    Response chan bool          // TUI 通过此 channel 返回确认结果
}

type ConfirmFunc func(req ConfirmRequest) bool
type PhaseFunc  func(phase string)
```

`BotInterface`（定义在 `tui/model.go`）约束 TUI 需要的 bot 能力：

```go
type BotInterface interface {
    RunAgent(input string, onStep func(...)) (string, error)
    ExecuteCommand(input string) (string, bool)
    TokenUsage() (int, int)
    CommandNames() []string
    SetConfirmFn(types.ConfirmFunc)
    SetPhaseFn(types.PhaseFunc)
    Provider() string
    Model() string
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

### 核心类型

| 类型 | 用途 |
|------|------|
| `Message` | 统一消息格式（Role / Content / ReasoningContent / ToolCalls / ToolCallID） |
| `ToolCall` | 结构化函数调用（ID + Function.Name + Function.Arguments） |
| `ToolDef` | 工具定义（FunctionDef + Parameters + Properties） |
| `Response` | LLM 响应（Choices + Usage） |
| `StreamChunk` | 流式 SSE 数据块 |

### Provider 适配

| Provider | 实现文件 | 特殊处理 |
|----------|---------|---------|
| OpenAI | `openai_compat.go` | `/chat/completions`，原生 tools |
| GLM 智谱 | `openai_compat.go` | 同 OpenAI 格式，默认 BaseURL 不同，支持 `reasoning_content` |
| DeepSeek | `openai_compat.go` | 同 OpenAI 格式，支持 `reasoning_content` |
| Anthropic | `anthropic.go` | `/v1/messages`，ContentBlock↔Message 双向转换 |

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
LevelSafe        (0) — 只读，自动放行
LevelWrite       (1) — 写操作，弹框确认
LevelDestructive (2) — 破坏操作，弹框确认
LevelForbidden   (3) — 永远禁止
```

### 内建工具

| 工具 | 文件 | 说明 |
|------|------|------|
| BashTool | `tool_bash.go` | Shell 命令执行，三级关键词匹配分级 |
| FileSystemTool | `tool_filesystem.go` | 文件 read/write/list |
| GlobTool | `tool_glob.go` | 文件模式匹配 |
| GrepTool | `tool_grep.go` | ripgrep 内容搜索，支持 regex/glob/context |
| EditTool | `tool_edit.go` | 精确字符串替换，失败返回文件内容 |

### 工具调用协议

`ParseCall(input) → (name, args, error)` 解析 `"toolName:key1=val1,key2=val2"` 格式。通过 `SplitPairs()` 处理逗号分隔，`unquote()` 处理引号转义。

## 确认机制

```
Agent goroutine                    TUI goroutine
  │                                  │
  ├─ executeTool()                   │
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

`Bot.SetConfirmFn()` 设置回调，Agent 调用时阻塞，TUI 通过 goroutine + channel 接收请求，渲染确认栏，用户按键后返回结果。

## TUI 组件树

```
Model
├── Header         — (^.^=) PRIMUS v0.1.0 · tokens · provider/model
├── Splash         — 启动页 (ASCII 猫 + 猫眼闪烁)
├── Messages       — 消息列表（基于 List 虚拟滚动）
│   ├── UserMessageItem        — 金色竖线 "You"
│   ├── AssistantMessageItem   — 绿色竖线 "Assistant" + ContentBlock 列表
│   ├── SystemMessageItem      — 蓝色竖线 "·"
│   ├── ErrorMessageItem       — 红色竖线 "!"
│   └── ProcessingItem         — 绿色竖线 "◉" + spinner + 流式块
├── Suggestions    — 斜杠命令自动补全
├── Input          — 消息输入框（历史翻阅、tab 补全）
└── ConfirmBar     — 工具确认卡片
```

### 结构化内容块 (ContentBlock)

Assistant Message 的内容由 `[]ContentBlock` 表示，替代旧的纯文本流：

```go
type ContentBlock struct {
    Type      BlockType  // BlockToolCall / BlockThinking / BlockText
    Content   string     // 渲染文本
    ToolName  string     // 工具名（仅 BlockToolCall）
    ToolArgs  string     // 简要参数
    Collapsed bool       // 折叠状态
}
```

- `ctrl+e` 翻转最后一条 assistant 消息中所有 tool block 的 Collapsed
- 折叠时显示 `◆ toolName args [+]`，展开时显示边框卡片 + 内容 + `[-]`

### TUI 消息流

```
tea.Batch(
    m.Spinner.Tick,           // spinner 动画
    listenConfirm(confirmCh), // 确认请求监听
    func() tea.Msg {          // Agent 执行 (goroutine)
        m.Bot.RunAgent(input, callback)
        return doneMsg{content, err}
    },
)
```

- agent 回调创建 `ContentBlock` 追加到 `BlockStream`
- `handleSpinnerTick` 从 `BlockStream` 读取块，更新 `ProcessingItem`
- `handleDone` 将块快照存入 `AssistantMessageItem`，清除处理状态

### Token 实时显示

```
(=^.^=) PRIMUS v0.1.0 · 1.2k/64k · openai/gpt-4
                        ───────
                        灰色(<60%) / 黄色(60-90%) / 红色(≥90%)
```

## 上下文管理

`ctxmgr.Manager` 实现 **Hybrid Window + Summary**：

- `Build(withTools)` 组装：system prompt → 摘要 → 滑动窗口消息 → 工具指令
- token 预算默认 64000，可在 config.json 中配置 `token_budget`
- 窗口大小 20 条消息，超出时 `Summarize()` 将前一半压缩为摘要
- 触发条件：消息 > 20 且 token > budget/2

## 配置

`~/.primusbot/config.json`：

```json
{
  "provider": "openai",
  "api_key": "sk-...",
  "model": "gpt-4",
  "base_url": "https://api.openai.com/v1",
  "token_budget": 64000
}
```

支持 provider: `openai` / `anthropic` / `glm`。
