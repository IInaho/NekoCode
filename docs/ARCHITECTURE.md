# PrimusBot 架构文档

## 项目概述

PrimusBot 是一个基于 Go 的终端 AI 助手，使用 Bubble Tea v2 构建 TUI，支持多 LLM provider（OpenAI / Anthropic / GLM / DeepSeek），具备 Agent 循环、Native Function Calling、工具执行、权限确认和上下文管理机制。

## 目录结构

```
primusbot/
├── main.go                         # 入口：无参→TUI 交互模式，有参→单次 CLI 模式
├── llm/                            # LLM 抽象层
│   ├── llm.go                      #   LLM 接口、Message/Response/ToolDef 等核心类型
│   ├── openai_compat.go            #   OpenAI 兼容实现（OpenAI / GLM / DeepSeek）
│   ├── anthropic.go                #   Anthropic 实现（tool_use/tool_result 双向转换 + SSE 流式）
│   └── event_reader.go             #   SSE 流解析器
├── bot/                            # 核心逻辑
│   ├── bot.go                      #   Bot 结构体、依赖注入、公开 API
│   ├── config.go                   #   配置加载 + 斜杠命令系统
│   ├── prompt/system.md            #   [embed] 角色 system prompt + 工具使用规则
│   ├── types/types.go              #   共享类型：ConfirmRequest / ConfirmFunc / PhaseFunc / Phase 常量
│   ├── extensions/                 #   插件接口
│   │   └── extensions.go           #     Extension 接口：Tools() + Commands()
│   ├── ctxmgr/                     #   上下文管理
│   │   ├── manager.go              #     Manager struct + Build() 上下文组装 + 统计
│   │   ├── storage.go              #     消息存取：Add/AddToolResult/AddToolResultsBatch/Clear
│   │   ├── token.go                #     Token 估算：语言感知（CJK ~1.5/token, ASCII ~4/token）
│   │   └── summarize.go            #     结构化摘要：NeedsSummarization / Summarize / BuildPrompt
│   ├── tools/                      #   工具系统（每工具一文件）
│   │   ├── tool.go                 #     Tool 接口、Registry、ExecutionMode、ParseCall、StripAnsi
│   │   ├── tool_bash.go            #     BashTool — Shell 命令执行 + 四级危险分级
│   │   ├── tool_read.go            #     ReadTool — 文件读取
│   │   ├── tool_write.go           #     WriteTool — 文件写入
│   │   ├── tool_edit.go            #     EditTool — 精确字符串替换 + diff 输出
│   │   ├── tool_list.go            #     ListTool — 目录列表
│   │   ├── tool_glob.go            #     GlobTool — 文件模式匹配（含 ** 递归）
│   │   ├── tool_grep.go            #     GrepTool — ripgrep 内容搜索
│   │   ├── tool_task.go            #     TaskTool — 子 agent 委派
│   │   ├── tool_todo.go            #     TodoWriteTool — 任务列表更新
│   │   ├── tool_webfetch.go        #     WebFetchTool — HTTP GET + HTML→Markdown
│   │   ├── tool_websearch.go       #     WebSearchTool — Exa MCP 优先 + Bing HTML 降级
│   │   └── html2md.go              #     HTML→Markdown 转换器
│   └── agent/                      #   Agent 循环
│       ├── agent.go                #     Agent 结构体、token 统计、Steer/Abort、replaceCtx
│       ├── run.go                  #     主循环 (Reason→Execute→Feedback) + BTW 中断
│       ├── reasoner.go             #     LLM 调用、流式 token 处理、工具调用解析
│       ├── executor.go             #     批量工具调度（串行/并行）、DangerLevel 检查、确认回调
│       ├── retry.go                #     LLM 调用指数退避重试（0.5s→8s，最多4次）
│       └── subagent/               #   子 agent 引擎
│           ├── engine.go           #     独立子 agent 循环（不依赖主 agent 包）
│           ├── registry.go         #     AgentType 定义 + 注册机制
│           └── agents.go           #     内置子 agent 定义（executor/verify/explore/plan/decompose）
└── tui/                            # 终端 UI（Bubble Tea v2）
    ├── tui.go                      #   Run() 入口
    ├── types.go                    #   BotInterface、ChatState、消息类型
    ├── model.go                    #   Model 结构体 + 初始化 + 状态切换
    ├── update.go                   #   tea.Update 消息分发
    ├── view.go                     #   tea.View 视图组装
    ├── phase.go                    #   处理阶段常量 + setPhase
    ├── agent.go                    #   startChat/startAgent/runAgent/onAgentStep
    ├── helpers.go                   #   工具参数格式化（formatBriefArgs）
    ├── handlers_spin.go            #   spinner tick + 流式消息回调
    ├── handlers_done.go            #   agent 完成后处理 + token 更新
    ├── handlers_keys.go            #   按键处理 + 确认键 + 调试日志 + suggestion 辅助
    ├── components/                 #   UI 组件
    │   ├── block/                  #   package block — 内容块类型与渲染
    │   │   ├── block.go            #     BlockType 枚举、ContentBlock 结构体
    │   │   ├── block_tool.go       #     工具调用行渲染（◆ edit foo.go [+]）
    │   │   ├── block_diff.go       #     diff 高亮渲染（-/+ 行着色）
    │   │   ├── block_text.go       #     文本块渲染（Thought / Reason）
    │   │   ├── block_filter.go     #     FilterFinalBlocks
    │   │   └── block_render.go     #     RenderBlock 分发器 + RenderBlocks 卡片包裹
    │   ├── message/                #   package message — 消息类型与渲染
    │   │   ├── message.go          #     ChatMessage 类型
    │   │   ├── message_shared.go   #     共享 helper（cachedRender, thickLeftBar）
    │   │   ├── message_user.go     #     UserMessageItem
    │   │   ├── message_assistant.go #    AssistantMessageItem
    │   │   ├── message_system.go   #     SystemMessageItem
    │   │   └── message_error.go    #     ErrorMessageItem
    │   ├── processing/             #   package processing — 流式渲染
    │   │   ├── processing.go       #     ProcessingItem 结构体 + mutator + Height
    │   │   ├── processing_render.go #    Render 编排器 + 5 个 section 方法
    │   │   └── text.go             #     RenderFixed / WrapPlain / IsNoiseLine
    │   ├── messages.go             #   Messages 容器 + AddMessage 分发
    │   ├── list_widget.go          #   List：通用滚动列表 + Item 接口
    │   ├── input.go                #   Input：textarea + 发送历史
    │   ├── header.go               #   Header：猫脸 + 应用名 + token 用量 + provider
    │   ├── splash.go               #   Splash：ASCII 猫 + 猫眼闪烁
    │   ├── confirm_bar.go          #   ConfirmBar：工具确认卡片
    │   ├── suggestions.go          #   Suggestions：斜杠命令自动补全
    │   ├── scrollbar.go            #   Scrollbar：独立滚动指示器
    │   └── todo_list.go            #   TodoList：任务列表
    └── styles/                     #   样式
        ├── colors.go               #     色彩体系 + Styles 结构体 + FmtTokens
        ├── charset.go              #     制表符字符集（含 ASCII 回退）
        └── markdown.go             #     glamour 封装（tokyo-night 主题）
```

## 包依赖图

```
main
  ├── bot ──────┬── bot/types ─── bot/tools
  │             ├── bot/extensions ─── bot/tools
  │             ├── bot/agent ───┬── bot/tools
  │             │                ├── bot/types
  │             │                ├── bot/ctxmgr ─── llm
  │             │                ├── llm
  │             │                └── bot/agent/subagent ─── bot/tools + bot/ctxmgr + llm
  │             └── ctxmgr
  └── tui ──────┬── bot (BotInterface)
                ├── components/block ─── styles
                ├── components/message ─── block + styles
                ├── components/processing ─── block + styles
                ├── components/ ──┬── block + message + processing
                │                └── styles
                └── styles (stdlib + lipgloss + glamour)
```

- `subagent` 包独立于 `agent` 包（不 import agent，避免循环依赖）
- `components/message` 和 `components/block` 互不依赖
- `components/processing` 依赖 `block` 和 `styles`
- `components/` 是唯一的组装者，导入 `block`、`message`、`processing` 三个子包

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
│  │         ├─ Content token → phaseFn("Reasoning") │ │
│  │         ├─ 返回 tool_calls → 解析        │ │
│  │         ├─ 保存 ReasoningContent         │ │
│  │         └─ 无 tool_calls → 直接文本回复   │ │
│  │                                          │ │
│  │  ② ExecuteBatch(calls) → results         │ │
│  │     ├─ 并行 ctx 取消检查                  │ │
│  │     ├─ needsSequential? → 串行/并行      │ │
│  │     ├─ phaseFn("Running " + name) → 工具名│ │
│  │     ├─ 检查 DangerLevel                  │ │
│  │     ├─ LevelWrite+ → confirmFn 确认       │ │
│  │     └─ tool.Execute() 执行工具            │ │
│  │                                          │ │
│  │  ③ Feedback(state, result)              │ │
│  │     ├─ step++ / shouldStop               │ │
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
    Input          string
    PreviousAction string
    PreviousOutput string
    Success        bool
    RetryCount     int
    SearchCount    int
    FetchCount     int
    lowOutputTurns int
}
```

### BTW 中断与 Steer 机制

Agent 处理中用户可输入新消息打断当前 LLM 调用并注入到上下文：

```
┌─── TUI 层 ──┐     ┌─── Agent 层 ──┐
│ Enter 按键   │     │                │
│ m.Bot.Steer()│────→│ steeringCh ← msg
│              │     │ replaceCtx()   │  ← 原子取消旧 ctx + 创建新 ctx（保留 parentCtx 链）
│ status 更新   │     │                │
└──────────────┘     │ drainSteering()│  ← 拾取消息注入 ctxMgr
                     │ continue       │  ← 新 Reason() 处理
                     └────────────────┘
```

- **Enter** → `Steer()`：注入消息 + 中断当前 LLM 调用
- **Esc** → `Abort()`：设置 `finished=true` + 取消 ctx
- `callLLMForTool` 检测 `context.Canceled` → 返回 `Interrupted` 标记
- `replaceCtx()` 使用 `parentCtx` 保持取消链，不丢失原始上下文

### 指数退避重试

`retry.go`：LLM 调用失败时自动重试，指数退避 0.5s→1s→2s→4s→8s（最多 4 次）。token 统计使用 `firstAttempt` 标记防止重复累加。

## 共享类型与接口

### BotInterface (tui/types.go)

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
    SetReasoningStreamFn(fn func(delta string))
    WireTodoWrite(fn tools.TodoFunc)
    SetCtxTodos(text string)
    Provider() string
    Model() string
}
```

### Phase 常量 (bot/types/types.go)

```go
const (
    PhaseReady     = "Ready"
    PhaseWaiting   = "Waiting"
    PhaseThinking  = "Thinking"
    PhaseReasoning = "Reasoning"
    PhaseRunning   = "Running"
)
```

Agent 侧和 TUI 侧统一引用 `types.Phase*`，保持单点定义。

## LLM 抽象层

### 接口

```go
type LLM interface {
    Chat(ctx, messages []Message, tools []ToolDef) (*Response, error)
    ChatStream(ctx, messages []Message, tools []ToolDef) (<-chan StreamToken, <-chan error)
    SetAPIKey(apiKey string)
    SetBaseURL(url string)
    SetMaxTokens(n int)
    MaxTokens() int
    SetDisableThinking(disable bool)
}
```

### Provider 适配

| Provider | 实现文件 | 特殊处理 |
|----------|---------|---------|
| OpenAI / GLM / DeepSeek | `openai_compat.go` | `/chat/completions`，thinking 参数控制 |
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
| `ModeParallel` | 可并发执行 | glob, grep, web_search, web_fetch, read, list, task, todo_write |
| `ModeSequential` | 独占执行 | bash, edit, write |

### 危险等级

```
LevelSafe        (0) — 只读，自动放行
LevelWrite       (1) — 写操作，弹框确认（bash 命令默认此级别）
LevelDestructive (2) — 破坏操作，弹框确认
LevelForbidden   (3) — 永远禁止
```

### 路径安全

`write` 和 `edit` 工具通过 `validatePath()` 校验路径不逃逸工作目录（拒绝 `..` 穿越）。

## 确认机制

```
Agent goroutine                    TUI goroutine
  │                                  │
  ├─ executeOne()                    │
  ├─ phaseFn("Running " + name)      │  ← phase 在确认前设置
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
│   ├── AssistantMessageItem   — teal ▐ 粗条 "Assistant" + ContentBlock + Footer
│   ├── SystemMessageItem      — 蓝色 ▐ 粗条 "·"
│   ├── ErrorMessageItem       — 红色 ▐ 粗条 "!"
│   └── ProcessingItem         — teal │ ◉ spinner + Phase + Blocks
├── Suggestions    — 斜杠命令自动补全
├── Input          — 消息输入框（历史翻阅、tab 补全）
└── ConfirmBar     — 工具确认卡片
```

### 视觉方案（厚左色条）

每条消息用 `▐`（U+2590）厚色条 + `PaddingLeft(1)` 统一缩进。

| 角色 | 色条颜色 |
|------|---------|
| User | 暖金 `#c9a96e` |
| Assistant | teal `#4ec9b0` |
| System | 蓝色 `#7a8ba0` |
| Error | 红色 `#e06c75` |

### 内容块 (ContentBlock)

```go
type ContentBlock struct {
    Type       BlockType  // BlockTool / BlockThought / BlockReason
    Content    string     // 渲染文本（edit 工具的 diff 嵌入此处）
    ToolName   string     // 工具名
    ToolArgs   string     // 简要参数
    Collapsed  bool       // 折叠状态
    BatchIdx   int        // 并行批次序号
    BatchTotal int        // 并行批次总数
}
```

- **BlockTool**: 暖金色卡片。仅 `edit` 工具显示 `[+]`/`[-]` 折叠，展开后渲染 diff（+/- 行着色）。所有块首行统一 2 字符缩进
- **BlockThought/BlockReason**: `💭` 前缀 + Subtle/Muted 色，首行前缀对齐
- `ctrl+e` 翻转所有 edit 工具块的 Collapsed

### 流式渲染

处理中显示结构化卡片（工具 + output/reasoning 分区），output 和 reasoning 区块动态高度（2-6 行），分隔线横跨全宽。最终回答一次 glamour 渲染。

### TUI 消息流

```go
tea.Batch(
    spinnerTick(),
    listenConfirm(m.confirmCh),
    m.runAgent(value),
)
```

- `tool_start` 回调 → 立即创建 collapsed BlockTool（所有工具）
- `execute_tool` 回调 → 仅 edit 工具追加 diff 内容
- `chat` 回调 → 设置 finalResponse + BlockThought
- `handleDone` 滤掉 ephemeral 块，保留 BlockTool 在最终消息

## 上下文管理

核心策略：**Hybrid Window + Structured Summary**。窗口大小 20，token 预算默认 64000。

```
[目标] 用户正在完成的目标
[进展] 已完成 / 进行中 / 阻塞
[关键决策] 技术选型、架构决策
[下一步] 待执行的操作
[关键上下文] 用户偏好、约束条件
[相关文件] 关键文件路径及作用
```

- 增量更新：已有摘要时在原基础上增删改
- Build() 保护 tool_calls/tool_result 配对不被截断
- 语言感知 token 估算（CJK ~1.5/token, ASCII ~4/token）

## Markdown 渲染

`glamour` 封装（tokyo-night 主题 + document margin=0 + 全宽度预热 40-160）。

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
