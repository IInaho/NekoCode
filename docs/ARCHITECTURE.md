# PrimusBot 架构文档

> **本文档职责**: 描述项目架构——目录结构、包依赖、模块实现、代码层面的机制。不包含 UI 设计、交互设计、设计原则等属于 DESIGN.md 的内容。更新时请保持此边界。

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
│   │   ├── truncate.go              #     工具输出截断 (2000行/50KB)
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
│  │     ├─ read-before-write check           │ │
│  │     ├─ tool.Execute() → TruncateOutput() │ │
│  │     └─ MarkRead() tracking               │ │
│  │                                          │ │
│  │  ③ Feedback(state, result)              │ │
│  │     ├─ step++ / shouldStop               │ │
│  │     ├─ doomLoopCheck (3x same → stop)    │ │
│  │     ├─ detectDiminishingReturns          │ │
│  │     └─ 构建下一步 stepState              │ │
│  │                                          │ │
│  │  ShouldStop? / doomLoop? → forceSynthesize()│ │
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

`Build()` 按顺序注入四层系统消息：system prompt → `[Task progress]` todo 文本 → `[Summary]` 摘要 → 消息历史。然后应用 20 条消息滑动窗口，按 token 预算从头部修剪。孤儿 tool 消息和无引用 tool_call_id 的消息被过滤。当 `withTools=true` 时，末尾追加工具选择提醒（"When the user asks you to perform actions, select the right tool..."）。

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
LevelSafe        (0) — 只读，自动放行 (标签: safe)
LevelWrite       (1) — 可能修改，确认 (标签: modify)
LevelDestructive (2) — 破坏操作，确认 (标签: danger)
LevelForbidden   (3) — 永远拒绝 (标签: blocked)
```

bash 命令智能分级：匹配 `go version`、`git log`、`git diff`、`ls`、`cat`、`ps` 等纯输出命令时返回 `LevelSafe`。

### 路径安全

`write` 和 `edit` 通过 `validatePath()` 解析符号链接并返回绝对路径。跨工作目录的路径不再被拒绝——确认系统处理用户同意。

## 幻觉防治体系

基于 Claude Code / OpenCode 的纵深防御思想，构建了 9 层防幻觉体系：

| 层 | 机制 | 位置 |
|----|------|------|
| 0 物理锚定 | 输出截断 + 先读后改强制 + 二进制检测 | `tools/truncate.go`, `executor.go`, `tool_read.go` |
| 1 提示约束 | 反幻觉指令 + 日期注入 + 来源引用 + 推理长度限制 | `prompt/system.md`, `bot.go` |
| 2 独立验证 | verify agent 格式强制 + 自检清单 + 反子 agent 提示 | `subagent/agents.go` |
| 3 记忆漂移 | 模板警告 + "当前现实优先"指令 | `session/memory_template.md` |
| 4 外部溯源 | web_search/fetch 强制引用 + 125字符限制 | `tool_websearch.go`, `tool_webfetch.go` |
| 5 上下文保真 | Critical Preservation Rules (代码/错误/路径) | `ctxmgr/summarize.go` |
| 6 格式锚定 | 文件未找到智能建议 (Levenshtein) | `tool_read.go` |
| 7 错误边界 | 末日循环检测 + Edit 模糊匹配 (3轮) + finish_reason=length 两级升级 | `agent/run.go`, `tool_edit.go`, `agent/reasoner.go` |
| 8 思考控制 | Anthropic adaptive 模式 + DeepSeek 默认关 thinking | `llm/anthropic.go`, `llm/openai_compat.go`, `bot/bot.go` |

### 思考模式管理

基于 Claude Code 源码分析：`getAPIProvider()` 返回 `firstParty` 导致 DeepSeek 被当作 Anthropic 内部模型，`modelSupportsAdaptiveThinking` 默认 `true`，最终发送 `thinking: {type: "adaptive"}`。DeepSeek 不认识 `adaptive` → 不开启思考。

- **Anthropic 端点** (`llm/anthropic.go`): 默认 `thinkingType: "adaptive"`，`SetThinkingBudget(N)` 切到 `enabled`（budget_tokens 分离），`SetDisableThinking(true)` 切到 `disabled`
- **OpenAI 兼容端点** (`llm/openai_compat.go`): 默认 `SetDisableThinking(true)`，发送 `{"thinking": {"type": "disabled"}}`
- **finish_reason=length 两级升级** (`agent/reasoner.go`): Tier 1 提升 max_tokens 到 64000；Tier 2 调用 `SetDisableThinking(true)` 重试
- **reasoning token 统计**: `token.ReasoningContent` 计入 `estCompl++` + `AddTokens(0,1)`，与 content token 统一统计

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

`BuildToolGroups()` 将连续同名 `BlockTool` 分组为 `ToolGroupInfo`。`renderGroupLine()` 渲染组头 `◆ name ×N [+]`，edit 组展开时调用 `RenderEditGroupExpanded()` 内联每个文件的 diff（无嵌套折叠）。其他工具组展开时逐条渲染各子块并缩进 2 格。

Ctrl+E 触发 `toggleBlocks()`：检测可折叠组/独立 edit 块，取反 `Collapsed` 状态。`BuildToolGroups` 和 `RenderEditGroupExpanded` 在 `block_render.go` 和 `processing_render.go` 两侧共享。

### 输出噪声过滤

`processing/text.go` 的 `isEmptyOrNoise()` 检测纯空白、纯点号、纯符号行。全噪声时 `renderOutputSection()` 跳过渲染。

## 对话中的 IO

### BTW 中断

Agent 处理中用户可输入新消息打断当前 LLM 调用并注入上下文。`replaceCtx()` 使用 `parentCtx` 保持取消链。

### ShouldStop 断路器

`bot.go` 注册 `SetShouldStop`：当 `searchCount >= 4 && fetchCount == 0` 时强制停止（搜索后从未抓取，模型可能在"想象"搜索结果）。同时 `detectDiminishingReturns`（连续 3 回合工具输出 < 200 字符）和 `doomLoopCheck`（3 次连续相同工具调用）触发 `forceSynthesize()`。

### ContextTransform

`bot.go` 注册 `SetContextTransform`：当工具结果 > 20 条时注入 `[System] N tool results accumulated...` 提示，引导模型检查未完成子任务或调用 verify 验证。

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
    SetThinkingBudget(tokens int)     // 0=default, -1=disabled, >0=custom (Anthropic)
    SetReasoningEffort(effort string) // "high"/"max" (OpenAI compat)
}
```

### Provider 适配

| Provider | 实现文件 | 要点 |
|----------|---------|------|
| OpenAI / GLM / DeepSeek | `openai_compat.go` | `/chat/completions`，`thinking: {type: "disabled"}` 默认关闭，`reasoning_effort` 控制 |
| Anthropic | `anthropic.go` | `/v1/messages`，SSE 解析，默认 `thinking: {type: "adaptive"}`，支持 `budget_tokens` |

## 模块职责

| 模块 | 位置 | 职责 |
|------|------|------|
| **Agent 循环** | `bot/agent/` | Reason→Execute→Feedback，BTW 中断，指数退避重试 |
| **子 Agent** | `bot/agent/subagent/` | 独立循环，thinking 禁用，共享 tools.Executor |
| **LLM 网关** | `llm/` | 统一对接多 provider，共享 HTTP 连接池，流式解析 |
| **工具系统** | `bot/tools/` | Tool 接口 + Executor + DangerLevel + 路径安全 + Phase 类型 |
| **上下文管理** | `bot/ctxmgr/` | 滑动窗口 + 微压缩 + 结构化摘要 + Snip |
| **Session Memory** | `bot/session/` | 异步 Markdown 提取，免费摘要 |
| **Bot 组装** | `bot/bot.go` | 依赖注入，ShouldStop，ContextTransform，session 接线 |
| **命令系统** | `bot/commands.go` | 斜杠命令解析与注册 |
| **配置** | `bot/config.go` | `~/.primusbot/config.json` 加载 |
| **TUI** | `tui/` | Bubble Tea v2，BotInterface 解耦，组件化 |

## 配置

`~/.primusbot/config.json`：

```json
{
  "provider": "openai",
  "api_key": "sk-...",
  "model": "gpt-4",
  "base_url": "https://api.openai.com/v1",
  "token_budget": 128000,
  "thinking_budget": 16000
}
```

`~/.primusbot/sessions/<id>/memory.md` — 自动创建的 session memory 文件。
