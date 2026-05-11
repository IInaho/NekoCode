# NekoCode 架构文档

> **本文档职责**: 描述项目架构——目录结构、包依赖、模块实现、代码层面的机制。不包含 UI 设计、交互设计、设计原则等属于 DESIGN.md 的内容。更新时请保持此边界。

## 项目概述

NekoCode 是一个基于 Go 的终端 AI 助手，使用 Bubble Tea v2 构建 TUI，支持多 LLM provider（OpenAI / Anthropic / GLM / DeepSeek），具备 Agent 循环、Native Function Calling、工具执行、权限确认、微压缩、Session Memory、Snip 消息剪枝和上下文管理机制。

## 目录结构

```
nekocode/
├── main.go                         # 入口：无参→TUI 交互模式，有参→单次 CLI 模式
├── llm/                            # LLM 抽象层
│   ├── llm.go                      #   LLM 接口、Message/Response/ToolDef 等核心类型
│   ├── openai_compat.go            #   OpenAI 兼容实现（OpenAI / GLM / DeepSeek）
│   ├── anthropic.go                #   Anthropic 实现（tool_use/tool_result 双向转换 + SSE 流式）
│   ├── event_reader.go             #   SSE 流解析器
│   └── retry.go                    #   指数退避重试（IsRetryable / Retry）
├── bot/                            # 核心逻辑
│   ├── bot.go                      #   Bot 结构体、依赖注入、公开 API
│   ├── config/                     #   配置管理
│   │   └── config.go               #     Config 结构体 + Load()（~/.nekocode/config.json）
│   ├── command/                    #   斜杠命令系统
│   │   └── parser.go               #     Parser + Command + Callbacks + RegisterDefaults
│   ├── prompt/system.md            #   [embed] system prompt：人设 + 工具规则
│   ├── session/                    #   Session Memory
│   │   ├── memory.go               #     Memory 结构体 + Extractor（异步提取）
│   │   └── memory_template.md      #     [embed] 10 section 结构化笔记模板
│   ├── context/                    #   项目上下文
│   │   └── project.go              #     NEKOCODE.md 发现 + @include 递归加载
│   ├── skill/                      #   Skill 系统
│   │   ├── skill.go                #     Registry + Workflow 类型 + 运行时引擎
│   │   ├── discovery.go            #     .nekocode/skills/ 目录发现
│   │   ├── inject.go               #     技能工作流注入 system prompt
│   │   ├── loader.go               #     YAML 技能加载器
│   │   ├── tool_skill.go           #     /skill 工具供 Agent 调用
│   │   └── bundled/                #     内置 Skill（编译进二进制）
│   │       ├── bundled.go          #       go:embed 加载 bundled/meta/SKILL.md
│   │       └── meta/SKILL.md       #       内置 Skill 定义
│   ├── ctxmgr/                     #   上下文管理
│   │   ├── manager.go              #     Manager 结构体 + Build() 上下文组装 + 统计
│   │   ├── compact.go              #     微压缩：token 紧张时清除旧工具结果
│   │   ├── anchor.go               #     上下文锚点：压缩时保留关键用户指令和系统约束
│   │   ├── auto_compact.go         #     五级预警自动压缩看门狗
│   │   ├── summarize.go            #     结构化摘要：NeedsSummarization/Summarize/BuildPrompt
│   │   ├── summarize_verify.go     #     摘要验证：LLM 生成的摘要二次校验后写入
│   │   ├── storage.go              #     消息存取：Add/AddToolResult/Clear/FreshStart + BoltDB 持久化
│   │   └── token.go                #     Token 估算（CJK ~1.5/token, ASCII ~4/token）
│   ├── tools/                      #   工具系统核心（接口、注册、执行、安全）
│   │   ├── tool.go                 #     Tool 接口、Registry、DangerLevel、ExecutionMode
│   │   ├── util.go                 #     StripAnsi、TruncateByRune、validatePath、SplitPairs、NewToolHTTPClient
│   │   ├── truncate.go             #     工具输出截断（2000行/50KB）
│   │   ├── descriptor.go           #     ToToolDefs()：Descriptor → LLM ToolDef 转换（共享）
│   │   ├── confirm.go              #     ConfirmRequest、ConfirmFunc、PhaseFunc、Phase 常量
│   │   ├── executor.go             #     Executor：并行/串行调度、危险分级检查、用户确认
│   │   ├── guard.go                #     安全门控：敏感工具操作确认提示 + 注入检测
│   │   ├── file_cache.go           #     FileStateCache：LRU + mtime 去重，跨子 Agent 共享
│   │   └── builtin/                #     内置工具实现（每工具一文件）
│   │       ├── register.go         #       RegisterAll()：注册全部内置工具
│   │       ├── tool_bash.go        #       BashTool — Shell 命令 + 四级危险分级
│   │       ├── tool_read.go        #       ReadTool — 文件读取（文本/图片/PDF）
│   │       ├── tool_write.go       #       WriteTool — 文件创建/覆写
│   │       ├── tool_edit.go        #       EditTool — 精确字符串替换 + diff 输出
│   │       ├── tool_list.go        #       ListTool — 目录列表
│   │       ├── tool_glob.go        #       GlobTool — 文件模式匹配（支持 ** 递归）
│   │       ├── tool_grep.go        #       GrepTool — ripgrep 内容搜索
│   │       ├── tool_task.go        #       TaskTool — 子 agent 委派
│   │       ├── tool_todo.go        #       TodoWriteTool — 任务列表更新
│   │       ├── tool_snip.go        #       SnipTool — 模型主动剪枝旧消息
│   │       ├── tool_webfetch.go    #       WebFetchTool — HTTP GET + HTML→Markdown
│   │       ├── tool_websearch.go   #       WebSearchTool — Exa MCP 搜索
│   │       ├── html2md.go          #       HTML→Markdown 转换器
│   │       └── html2md_test.go     #       HTML→Markdown 测试
│   └── agent/                      #   Agent 循环
│       ├── agent.go                #     Agent 结构体、token 统计、Steer/Abort、replaceCtx
│       ├── log.go                  #     Debug 日志（/tmp/nekocode-debug.log）
│       ├── run.go                  #     主循环（Reason→Execute→Feedback）+ BTW 中断
│       ├── reasoner.go             #     LLM 调用、流式 token 处理、工具调用解析
│       ├── executor.go             #     ActionResult 类型
│       ├── retry.go                #     LLM 调用指数退避重试（0.5s→8s，最多4次）
│       └── subagent/               #     子 agent 引擎
│           ├── engine.go           #       独立子 agent 循环（共享 tools.Executor）
│           ├── registry.go         #       AgentType 定义 + 注册机制
│           └── agents.go           #       内置子 agent 定义（executor/verify/explore/plan/decompose）
└── tui/                            # 终端 UI（Bubble Tea v2）
    ├── tui.go                      #   Run() 入口
    ├── types.go                    #   BotInterface（18 方法）、chatState、doneMsg/confirmMsg
    ├── model.go                    #   Model 结构体 + 初始化 + 状态切换 + Phase 常量
    ├── update.go                   #   tea.Update 消息分发
    ├── view.go                     #   tea.View 视图组装
    ├── agent.go                    #   startChat/startAgent/runAgent/onAgentStep
    ├── handlers.go                 #   按键处理 + 完成处理 + spinner tick（合并原3文件）
    ├── helpers.go                  #   工具参数格式化（formatBriefArgs）
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
    │   │   ├── message_error.go    #     ErrorMessageItem
    │   │   └── markdown.go         #     glamour 封装（tokyo-night 主题，按宽度缓存）
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
    │   └── scrollbar.go            #   Scrollbar：独立滚动指示器
    └── styles/                     #   样式
        ├── colors.go               #     色彩体系 + Styles 结构体 + FmtTokens
        └── charset.go              #     制表符字符集（含 ASCII 回退）
```

## 包依赖图

```
main
  ├── bot ──────┬── config ──── (stdlib)
  │             ├── command ─── (stdlib)
  │             ├── session ──── ctxmgr + llm
  │             ├── context ─── (stdlib)
  │             ├── skill ──────┬── tools + ctxmgr + llm
  │             │               └── bundled ─── skill
  │             ├── agent ───┬── tools ─── llm
  │             │            ├── ctxmgr ─── llm
  │             │            ├── llm
  │             │            └── subagent ─── tools + ctxmgr + llm
  │             ├── tools ───┬── llm
  │             │            └── builtin ─── tools + llm
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
- `tools/builtin` 包含所有内置工具的具体实现，通过 `RegisterAll()` 注册
- `subagent` 与 `agent` 共享 `tools.Executor`，保证工具安全检查一致
- `config` 和 `command` 为独立子包，通过 `bot.go` 组装
- `session` 异步 goroutine 提取，依赖 `ctxmgr` 构建上下文
- `skill` 技能系统，YAML 定义工作流，运行时引擎执行；`bundled` 子包提供编译进二进制的内置技能
- `context` 项目上下文预加载，NEKOCODE.md 发现与 @include 递归
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

### 五级预警 + 自动压缩

`AutoCompactIfNeeded()` 在每次 `Build()` 前运行，根据剩余 token buffer 触发不同级别操作：

| Level | 剩余 buffer | 动作 |
|-------|------------|------|
| Normal | > 20,000 | 无操作 |
| Warning | ≤ 20,000 | 无操作（仅告警） |
| MicroCompact | ≤ 13,000 | 触发微压缩 |
| Compact | ≤ 10,000 | 触发完整压缩（Session Memory → LLM Summarize） |
| Blocking | ≤ 3,000 | 拒绝新输入，强制压缩 |

### 上下文锚点

`anchor.go` 在压缩前标记关键消息——包含用户核心指令、系统约束、API 版本要求等的消息在压缩时优先保留，防止关键信息被误清除。

### 摘要验证

`summarize_verify.go` 在 LLM 生成摘要后执行二次校验：检查摘要是否保留了代码片段、错误信息、文件路径等关键内容，验证失败则重新生成。

### 微压缩

`MicroCompactIfNeeded()` 在每次 `Build()` 前调用，但仅在 token 超过预算 50% 时激活。将旧的 compactable 工具结果（read、bash、grep、glob、web_search、web_fetch、edit、write）内容替换为 `[Old tool result cleared]`，始终保留最近 5 个结果。

防止大文件读取/命令输出导致上下文膨胀，同时模型随时可以重新执行工具获取内容。非 compactable 工具（task、todo_write、snip）永不清除。

### 结构化摘要

token 超过预算 80% 时，`Summarize()` 将最旧的一半消息压缩为结构化摘要，包含：Goal、Progress、Key Decisions、Next Steps、Critical Context、Relevant Files。支持已有摘要的增量更新。

### Session Memory

上下文超过 10k token 后，每隔 +5k token 和 3+ tool call 触发异步提取。goroutine 调用 LLM 更新 `~/.nekocode/sessions/<id>/memory.md`（10 section Markdown 文件）。`/new` 命令优先用 session memory 作为免费摘要。

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

### 文件缓存

`GlobalFileCache`（`tools/file_cache.go`）在多次 read 调用间缓存文件内容。LRU 驱逐策略（100 条目 / 25MB 上限），基于文件 mtime + size 校验自动失效。子 Agent 自动共享同一缓存实例。缓存命中时直接返回缓存内容，避免重复磁盘 I/O。

### 安全门控

`guard.go` 为敏感工具（bash、write、edit）提供统一的确认提示入口。根据工具和参数计算 DangerLevel，LevelWrite 及以上触发用户确认流程。

## 幻觉防治体系

基于纵深防御思想，在 6 个代码层面实现 43 个防幻觉机制（以下仅列出代表性机制）：

### 第 1 层：工具安全

| 机制 | 位置 |
|------|------|
| 危险等级四级分类（Safe/Write/Destructive/Forbidden） | `tools/tool.go` |
| bash 命令关键词智能分级（sudo/rm/kill → Forbidden/Destructive，ls/pwd → Safe） | `tools/builtin/tool_bash.go` |
| 路径验证 + 符号链接解析 | `tools/util.go` |
| 二进制文件检测（null 字节 + UTF-8 校验 + 可打印比例） | `tools/builtin/tool_read.go` |
| URL 校验 + 内网 IP 拒绝（SSRF 防护） | `tools/builtin/tool_webfetch.go` |
| WebFetch 重定向上限 5 次 | `tools/builtin/tool_webfetch.go` |
| Web 搜索结果上限 15 条 | `tools/builtin/tool_websearch.go` |

### 第 2 层：执行拦截

| 机制 | 位置 |
|------|------|
| DangerLevel 强制校验：LevelForbidden 直接拒绝，LevelWrite+ 需用户确认 | `tools/executor.go` |
| 先读后改强制：write/edit 前检查文件是否已 read，未读则拒绝 | `tools/executor.go` |
| 文件读取追踪（MarkRead/WasRead），跨子 Agent 共享 | `tools/executor.go` |
| 并行/串行分区调度 + worker pool 上限 10 | `tools/executor.go` |

### 第 3 层：输出完整性

| 机制 | 位置 |
|------|------|
| 工具结果边界标记（`--- BEGIN/END tool_result ---`） | `tools/guard.go` |
| Prompt Injection 检测（~30 个中英文指令模式，风险权重 1-3 分级） | `tools/guard.go` |
| 工具输出截断（2000行/50KB） | `tools/truncate.go` |
| Web 内容截断（WebFetch 3000 runes，WebSearch 6000 runes） | `tools/builtin/tool_webfetch.go`, `tool_websearch.go` |
| Garbled tool call 检测（截断 XML/JSON 片段过滤） | `agent/reasoner.go` |

### 第 4 层：Agent 循环控制

| 机制 | 位置 |
|------|------|
| 末日循环检测：连续 3 次相同工具调用 → forceSynthesize | `agent/run.go` |
| 收益递减检测：连续 3 回合工具输出 < 200 字符 → 强制停止 | `agent/run.go` |
| 断路器：searchCount ≥ 4 && fetchCount == 0 → 强制停止 | `bot/bot.go` |
| ContextTransform：工具结果 > 20 条时注入推进提示 | `bot/bot.go` |
| finish_reason=length 两级升级（Tier 1: max_tokens → 64000，Tier 2: 关 thinking） | `agent/reasoner.go` |
| LLM 调用指数退避重试（0.5s→8s，最多 4 次）+ 错误分类 | `agent/retry.go`, `llm/retry.go` |

### 第 5 层：上下文保真

| 机制 | 位置 |
|------|------|
| 关键约束锚定（10 个正则提取 "不要/必须/do not/must" 等，永不压缩） | `ctxmgr/anchor.go` |
| 当前目标锚定（提取首条实质性用户消息 + session memory） | `ctxmgr/anchor.go` |
| 每次 Add() 自动提取用户消息中的约束 | `ctxmgr/storage.go` |
| 摘要后约束验证 + 缺失时重新摘要（最多 1 次） | `ctxmgr/summarize_verify.go` |
| 摘要尾部保留最后 3 轮对话 | `ctxmgr/summarize.go` |
| 微压缩（50% 预算时清除旧工具结果，保留最近 5 条） | `ctxmgr/compact.go` |
| 五级自动压缩（Normal → Warning → Micro → Compact → Blocking） | `ctxmgr/auto_compact.go` |
| Build() 孤儿 tool 消息过滤 + 空内容兜底 | `ctxmgr/manager.go` |
| Token 估算（ASCII ~4/token, CJK ~1.5/token）+ API 校准 | `ctxmgr/token.go` |

### 第 6 层：LLM 调用控制

| 机制 | 位置 |
|------|------|
| Anthropic thinking budget 钳制（min(16000, maxTokens/2)，budget < maxTokens 强制） | `llm/anthropic.go` |
| OpenAI 兼容默认关 thinking（`{"thinking": {"type": "disabled"}}`） | `llm/openai_compat.go` |
| 子 Agent thinking 强制关闭（注释："Sub-agents execute — they don't need extended reasoning"） | `bot/bot.go` |
| finish_reason=length 时关 thinking 释放全部 token 给输出 | `agent/reasoner.go` |
| Anthropic 开启 thinking 时 temperature 强制设为 1 | `llm/anthropic.go` |

### 非代码层（prompt/设计级补充）

以下机制通过 system prompt 和子 agent 提示文本实现，属于设计级防护，不在此表中：

- System prompt 反幻觉指令（禁止生成 URL、忠实报告、先验证再声称完成）
- 日期注入（防止时间幻觉）
- 推理长度限制（bug fix 1 句、重构 3 句、设计 5 句）
- verify agent 格式强制 + 自检清单
- Session memory 模板警告（"记忆说 X 存在 ≠ X 现在存在"）
- web_search/fetch 的 Sources 引用格式要求（prompt 文本，非代码强制）

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

### Markdown 渲染

`tui/components/message/markdown.go` 封装 glamour 库，使用 tokyo-night 主题。按终端宽度缓存 renderer 实例（40-160 字符），`Warmup()` 预创建常用宽度的渲染器以加速首屏显示。

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

`llm/retry.go` 和 `agent/retry.go`：LLM 调用失败自动重试，0.5s→1s→2s→4s→8s（最多 4 次）。

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
| **内置工具** | `bot/tools/builtin/` | 12 个内置工具的具体实现，通过 RegisterAll() 注册 |
| **上下文管理** | `bot/ctxmgr/` | 五级预警 + 滑动窗口 + 微压缩 + 锚点 + 摘要验证 + Snip + BoltDB |
| **Session Memory** | `bot/session/` | 异步 Markdown 提取，免费摘要 |
| **Skill 系统** | `bot/skill/` | YAML 技能定义 + 发现 + 注入 + 运行时引擎 |
| **内置 Skill** | `bot/skill/bundled/` | 编译进二进制的内置技能（go:embed） |
| **项目上下文** | `bot/context/` | NEKOCODE.md 发现 + @include 递归加载 |
| **Bot 组装** | `bot/bot.go` | 依赖注入，ShouldStop，ContextTransform，session 接线 |
| **命令系统** | `bot/command/` | 斜杠命令解析与注册，Callbacks 模式解耦 |
| **配置** | `bot/config/` | `~/.nekocode/config.json` 加载 |
| **TUI** | `tui/` | Bubble Tea v2，BotInterface（18 方法）解耦，组件化 |

## Skill 系统

### 双层 Skill 来源

1. **Bundled Skills** (`bot/skill/bundled/`)：编译进二进制的内置技能，使用 `go:embed` 加载 `meta/SKILL.md`，始终可用
2. **File-based Skills** (`bot/skill/discovery.go`)：从 `.nekocode/skills/` 目录自动发现 YAML 技能定义

### 注册顺序

Bundled skills 优先注册，保证内置技能优先级高于文件系统技能。Skill 同时注册为斜杠命令（`/<skill-name>`）和工具（供 Agent 调用）。

### 技能上下文管理

Agent 调用技能时，技能内容注入为 user 消息。下一轮若不再需要该技能，自动通过 snip 清除技能消息，释放上下文空间。

## 配置

`~/.nekocode/config.json`：

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

`~/.nekocode/sessions/<id>/memory.md` — 自动创建的 session memory 文件。

BoltDB 数据库用于对话历史的持久存储（`bot/ctxmgr/storage.go`），确保重启后对话不丢失。
