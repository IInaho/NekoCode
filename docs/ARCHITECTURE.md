# PrimusBot 架构文档

## 项目概述

PrimusBot 是一个基于 Go 的终端 AI 助手，使用 Bubble Tea v2 构建 TUI，支持多 LLM provider（OpenAI / Anthropic / GLM / DeepSeek），具备 Agent 循环、Native Function Calling、工具执行、权限确认、微压缩、Session Memory、Snip 消息剪枝和上下文管理机制。

## 目录结构

```
primusbot/
├── main.go                         # 入口：无参→TUI 交互模式，有参→单次 CLI 模式
├── llm/                            # LLM 抽象层
│   ├── llm.go                      #   LLM 接口、Message/Response/ToolDef 等核心类型
│   ├── openai_compat.go            #   OpenAI 兼容实现（OpenAI / GLM / DeepSeek）
│   ├── anthropic.go                #   Anthropic 实现（tool_use/tool_result 双向转换 + SSE 流式）
│   ├── event_reader.go             #   SSE 流解析器
│   └── retry.go                    #   指数退避重试（IsRetryable / Retry）
├── bot/                            # 核心逻辑
│   ├── bot.go                      #   Bot 结构体、依赖注入、公开 API
│   ├── config.go                   #   配置加载（~/.primusbot/config.json）
│   ├── commands.go                 #   斜杠命令系统（Parser，/help，/new，/clear，/stats 等）
│   ├── prompt/system.md            #   [embed] system prompt：人设 + 工具规则
│   ├── session/                    #   Session Memory
│   │   ├── memory.go               #     Memory 结构体 + Extractor（异步提取）
│   │   └── memory_template.md      #     [embed] 10 section 结构化笔记模板
│   ├── ctxmgr/                     #   上下文管理
│   │   ├── manager.go              #     Manager 结构体 + Build() 上下文组装 + 统计
│   │   ├── compact.go              #     微压缩：token 紧张时清除旧工具结果
│   │   ├── storage.go              #     消息存取：Add/AddToolResult/Clear/FreshStart
│   │   ├── token.go                #     Token 估算（CJK ~1.5/token, ASCII ~4/token）
│   │   └── summarize.go            #     结构化摘要：NeedsSummarization/Summarize/BuildPrompt
│   ├── tools/                      #   工具系统（每工具一文件）
│   │   ├── tool.go                 #     Tool 接口、Registry、DangerLevel、ExecutionMode
│   │   ├── util.go                 #     StripAnsi、TruncateByRune、validatePath、SplitPairs、NewToolHTTPClient
│   │   ├── descriptor.go           #     ToToolDefs()：Descriptor → LLM ToolDef 转换（共享）
│   │   ├── confirm.go              #     ConfirmRequest、ConfirmFunc、PhaseFunc、Phase 常量
│   │   ├── executor.go             #     Executor：并行/串行调度、危险分级检查、用户确认
│   │   ├── tool_bash.go            #     BashTool — Shell 命令 + 四级危险分级
│   │   ├── tool_read.go            #     ReadTool — 文件读取（文本/图片/PDF）
│   │   ├── tool_write.go           #     WriteTool — 文件创建/覆写
│   │   ├── tool_edit.go            #     EditTool — 精确字符串替换 + diff 输出
│   │   ├── tool_list.go            #     ListTool — 目录列表
│   │   ├── tool_glob.go            #     GlobTool — 文件模式匹配（支持 ** 递归）
│   │   ├── tool_grep.go            #     GrepTool — ripgrep 内容搜索
│   │   ├── tool_task.go            #     TaskTool — 子 agent 委派
│   │   ├── tool_todo.go            #     TodoWriteTool — 任务列表更新
│   │   ├── tool_snip.go            #     SnipTool — 模型主动剪枝旧消息
│   │   ├── tool_webfetch.go        #     WebFetchTool — HTTP GET + HTML→Markdown
│   │   ├── tool_websearch.go       #     WebSearchTool — Exa MCP 搜索
│   │   ├── html2md.go              #     HTML→Markdown 转换器
│   │   └── html2md_test.go         #     HTML→Markdown 测试
│   └── agent/                      #   Agent 循环
│       ├── agent.go                #     Agent 结构体、token 统计、Steer/Abort、replaceCtx
│       ├── log.go                  #     Debug 日志（/tmp/primusbot-debug.log）
│       ├── run.go                  #     主循环 (Reason→Execute→Feedback) + BTW 中断
│       ├── reasoner.go             #     LLM 调用、流式 token 处理、工具调用解析
│       ├── executor.go             #     ActionResult 类型（Executor 已移入 tools）
│       ├── retry.go                #     LLM 调用指数退避重试（0.5s→8s，最多4次）
│       └── subagent/               #   子 agent 引擎
│           ├── engine.go           #     独立子 agent 循环（共享 tools.Executor）
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
    ├── helpers.go                  #   工具参数格式化（formatBriefArgs）
    ├── handlers_spin.go            #   spinner tick + 流式消息回调
    ├── handlers_done.go            #   agent 完成后处理 + token 更新
    ├── handlers_keys.go            #   按键处理 + 确认键 + suggestions
    ├── components/                 #   UI 组件
    │   ├── block/                  #   内容块类型与渲染
    │   │   ├── block.go            #     BlockType 枚举、ContentBlock 结构体
    │   │   ├── block_tool.go       #     工具调用行渲染（◆ read path [+]）
    │   │   ├── block_diff.go       #     diff 高亮渲染（-/+ 行着色）
    │   │   ├── block_text.go       #     文本块渲染（Thought / Reason）
    │   │   ├── block_filter.go     #     FilterFinalBlocks
    │   │   └── block_render.go     #     RenderBlock 分发器 + BuildToolGroups + RenderBlocks
    │   ├── message/                #   消息类型与渲染
    │   │   ├── message.go          #     ChatMessage 类型
    │   │   ├── message_shared.go   #     共享 helper（cachedRender, thickLeftBar）
    │   │   ├── message_user.go     #     UserMessageItem
    │   │   ├── message_assistant.go #    AssistantMessageItem
    │   │   ├── message_system.go   #     SystemMessageItem
    │   │   └── message_error.go    #     ErrorMessageItem
    │   ├── processing/             #   流式渲染
    │   │   ├── processing.go       #     ProcessingItem 结构体 + mutator + Height
    │   │   ├── processing_render.go #    Render 编排器 + 5 个 section 方法
    │   │   └── text.go             #     RenderFixed / WrapPlain / isEmptyOrNoise
    │   ├── messages.go             #   Messages 容器 + AddMessage 分发 + ToggleLastAssistant
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
  ├── bot ──────┬── session ──── ctxmgr + llm
  │             ├── agent ───┬── tools ─── llm
  │             │            ├── ctxmgr ─── llm
  │             │            ├── llm
  │             │            └── subagent ─── tools + ctxmgr + llm
  │             ├── tools ─── llm
  │             └── ctxmgr ─── llm
  └── tui ──────┬── bot (BotInterface)
                ├── components/block ─── styles
                ├── components/message ─── block + styles
                ├── components/processing ─── block + styles
                ├── components/ ──┬── block + message + processing
                │                └── styles
                └── styles (stdlib + lipgloss + glamour)
```

- `tools` 是整个系统的基础层：Tool 接口、Registry、Executor、Phase 类型、Confirm 类型
- `subagent` 与 `agent` 共享 `tools.Executor`，保证工具安全检查一致
- `session` 异步 goroutine 提取，依赖 `ctxmgr` 构建上下文
- `tui/components/block` 导出 `BuildToolGroups` 和 `ToolGroupInfo`，streaming 和 message 两边共用

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
│  │         ├─ MicroCompactIfNeeded()        │ │
│  │         ├─ ctxMgr.Build(true) 组装上下文  │ │
│  │         ├─ llmClient.ChatStream() 流式   │ │
│  │         ├─ withRetry() 指数退避重试       │ │
│  │         └─ 解析 tool_calls / text        │ │
│  │                                          │ │
│  │  ② ExecuteBatch(calls) → results         │ │
│  │     ├─ partition(ro, mw)                 │ │
│  │     ├─ runParallel(ro) / runSequential(mw)│ │
│  │     ├─ DangerLevel 检查 + confirmFn      │ │
│  │     └─ tool.Execute()                    │ │
│  │                                          │ │
│  │  ③ Feedback(state, result)              │ │
│  │     ├─ step++ / shouldStop               │ │
│  │     ├─ detectDiminishingReturns          │ │
│  │     └─ 构建下一步 stepState              │ │
│  │                                          │ │
│  │  ShouldStop? → forceSynthesize()         │ │
│  └──────────────────────────────────────────┘ │
│                                              │
│  返回 RunResult{FinalOutput, Steps}          │
└──────────────────────────────────────────────┘
```

## 上下文管理

### 微压缩

`MicroCompactIfNeeded()` 在每次 `Build()` 前调用，但仅在 token 超过预算 50% 时激活。将旧的 compactable 工具结果（read、bash、grep、glob、web_search、web_fetch、edit、write）内容替换为 `[Old tool result cleared]`，始终保留最近 5 个结果。

防止大文件读取/命令输出导致上下文膨胀，同时模型随时可以重新执行工具获取内容。非 compactable 工具（task、todo_write、snip）永不清除。

### 结构化摘要

token 超过预算 80% 时，`Summarize()` 将最旧的一半消息压缩为结构化摘要，包含：Goal、Progress、Key Decisions、Next Steps、Critical Context、Relevant Files。支持已有摘要的增量更新。

### Session Memory

上下文超过 10k token 后，每隔 +5k token 和 3+ tool call 触发异步提取。goroutine 调用 LLM 更新 `~/.primusbot/sessions/<id>/memory.md`（10 section Markdown 文件）。`/new` 命令优先用 session memory 作为免费摘要。

### Snip 剪枝

`snip` 工具让模型主动移除旧消息范围以节省上下文。`[id:N]` 标签仅在 API 侧注入（用户不可见）。被 snipped 的消息在后续 `Build()` 中被过滤。

### 滑动窗口 + Token 预算

`Build()` 先应用 20 条消息滑动窗口，再按 token 预算从头部修剪。孤儿 tool 消息和无引用 tool_call_id 的消息被过滤。

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
| `ModeParallel` | 可并发执行 | glob, grep, web_search, web_fetch, read, list, task |
| `ModeSequential` | 独占执行 | bash, edit, write, snip, todo_write |

### 危险等级

```
LevelSafe        (0) — 只读，自动放行
LevelWrite       (1) — 写操作，确认
LevelDestructive (2) — 破坏操作，确认
LevelForbidden   (3) — 永远拒绝
```

### 路径安全

`write` 和 `edit` 通过 `validatePath()` 校验路径不逃逸工作目录。

## TUI 组件树

```
Model
├── Header         — provider/model · ↑tokens ↓tokens 🧹N
├── Splash         — 启动页 (ASCII 猫 + 猫眼闪烁)
├── Messages + Scrollbar — 消息列表 + 独立滚动指示器
│   ├── UserMessageItem        — 暖金 ▐ 粗条 "You"
│   ├── AssistantMessageItem   — teal ▐ 粗条 "Assistant" + ContentBlocks + Footer
│   ├── SystemMessageItem      — 蓝色 ▐ 粗条 "·"
│   ├── ErrorMessageItem       — 红色 ▐ 粗条 "!"
│   └── ProcessingItem         — teal │ ◉ spinner + Phase + 工具组 + Output + Reasoning
├── Suggestions    — 斜杠命令自动补全
├── Input          — 消息输入框（历史翻阅、tab 补全）
└── ConfirmBar     — 工具确认卡片
```

### 工具组折叠

同名单行工具块（如 15 个并行 `read`）渲染为可折叠组：

```
◆ read ×15 [+] 展开    ← 收起（单行）
◆ read ×15 [-] 收起    ← 展开：
  ◆ read (1/15) /path/to/file1.go
  ◆ read (2/15) /path/to/file2.go
  ...
```

Ctrl+E 切换折叠/展开。`BuildToolGroups` 函数在 streaming 和 message 两侧共享。

### 输出噪声过滤

`isEmptyOrNoise()` 过滤纯空白、纯点号、纯符号行。所有内容都是噪声时整个 output 块不渲染。

## 对话中的 IO

### BTW 中断

Agent 处理中用户可输入新消息打断当前 LLM 调用并注入上下文。`replaceCtx()` 使用 `parentCtx` 保持取消链。

### 指数退避重试

`retry.go`：LLM 调用失败自动重试，0.5s→1s→2s→4s→8s（最多 4 次）。

### 确认机制

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

| Provider | 实现文件 | 要点 |
|----------|---------|------|
| OpenAI / GLM / DeepSeek | `openai_compat.go` | `/chat/completions`，thinking 参数控制 |
| Anthropic | `anthropic.go` | `/v1/messages`，SSE content_block_start/delta 解析 |

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

`~/.primusbot/sessions/<id>/memory.md` — 自动创建的 session memory 文件。
