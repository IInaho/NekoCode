# NekoCode 上下文管理架构

## 总览

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                            LLM API (DeepSeek / OpenAI / Anthropic)            │
│                                                                              │
│  输入：system prompt + messages[] + tool_defs                                 │
│  输出：text delta + reasoning delta + tool_call delta + usage                 │
│  约束：context window ≤ tokenBudget (默认 64K，可配)                           │
└───────────────────────────┬──────────────────────────────────────────────────┘
                            │
                            ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│                         ctxmgr.Manager (上下文管理器)                          │
│                                                                              │
│  核心数据结构：                                                               │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ systemPrompt (//go:embed)     ← 嵌入的猫娘角色 + 编码规范 + 工具规则   │   │
│  │ projectContext (NEKOCODE.md)  ← 会话启动一次性加载，同会话 memoized    │   │
│  │ todoText                      ← 每轮 LLM 调用前注入的 todo 进度        │   │
│  │ skillList                     ← 可用 slash command 列表               │   │
│  │ summary                       ← 压缩后的历史摘要                       │   │
│  │ messages[]                    ← 完整消息历史（含 compact boundary 前） │   │
│  │ snipped map                   ← 被 snip 工具标记删除的消息索引         │   │
│  │ compactBoundary               ← 压缩边界，之前 = 已摘要，之后 = 活跃   │   │
│  │ tokenBudget                   ← token 预算上限                         │   │
│  │ tokenTracker                  ← API 精确 + 估算混合计数                │   │
│  │ autoCompactCfg                ← 五级预警阈值                           │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  Build() 组装流程：                                                          │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ 1. systemPrompt (system)                                              │   │
│  │ 2. [Task progress] + todoText (system)                                │   │
│  │ 3. skillList (system)                                                 │   │
│  │ 4. [Summary] + summary (system)    ← 有则注入                          │   │
│  │ 5. messages[compactBoundary:]      ← 跳过已压缩区间                    │   │
│  │    ├─ sliding window 截取最后 windowSize 条                           │   │
│  │    ├─ token budget 超出时从前面丢弃                                    │   │
│  │    ├─ 过滤 snipped 索引                                              │   │
│  │    ├─ 过滤孤儿 tool_result                                           │   │
│  │    └─ user message 注入 [id:N] 标签（供 snip 引用）                    │   │
│  │ 6. Tool hint (system)              ← withTools=true 时追加            │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
└───────────────────────────┬──────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────────┐
        ▼                   ▼                       ▼
┌──────────────────┐ ┌──────────────┐ ┌──────────────────────┐
│  压缩层           │ │  缓存层       │ │  持久记忆层           │
│                  │ │              │ │                      │
│ 五级递进:        │ │ FileStateCache│ │ Session Memory       │
│ Normal→Warning→  │ │ LRU 100条目   │ │ .nekocode/sessions/  │
│ MicroCompact→    │ │ 25MB上限      │ │   <id>/memory.md    │
│ Compact→Blocking │ │ mtime去重     │ │                      │
│                  │ │ 跨子agent共享  │ │ 异步LLM提取          │
│ 路径:            │ │              │ │ 阈值触发              │
│ MicroCompact →   │ │ Get()/Put()/ │ │ 用作免费摘要          │
│ SessionMemory →  │ │ GetContent()/ │ │                      │
│ LLM Summarize    │ │ Merge()/Clone()│ │ 模板: Title/State/  │
│                  │ │              │ │ Task/Files/Errors/   │
│ 截断后 snipped   │ │              │ │ Learnings/Worklog    │
│ map 同步修复     │ │              │ │                      │
└──────────────────┘ └──────────────┘ └──────────────────────┘
```

## System Prompt 构建

```
┌──────────────────────────────────────────────────────────────┐
│ systemPrompt (//go:embed bot/prompt/system.md)               │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ 猫娘角色设定 + 语气风格                                │   │
│  │ 安全约束 + 诚实验证                                    │   │
│  │ 工具选择指南 (Read/Grep/Glob > Bash)                   │   │
│  │ 代码风格偏好                                           │   │
│  │ 输出效率规则                                           │   │
│  │ Sub-agent 委托标准                                     │   │
│  │ 幻觉防护设计                                           │   │
│  │ Skill 工作流 (已加载技能时注入)                         │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                              │
│  + <system-reminder> (user message, 不污染 system prompt)    │
│  │  CWD + 今日日期 + 探索提示                               │
│                                                              │
│  + <project-context> (system message, 来自 NEKOCODE.md)      │
│  │  加载链: ~/.nekocode/ → 项目根/ → .nekocode/ → rules/   │
│  │  支持 @include 递归 (max depth=3)                        │
│  │  上限 40K chars                                          │
│  │  @ 过滤: only paths with / or known extension            │
│                                                              │
│  + [Task progress] (system, 每轮注入)                        │
│  + skillList (system, 每轮注入)                              │
│  + [Summary] (system, 有压缩摘要时注入)                      │
│  + Tool hint (system, withTools=true 时追加)                 │
└──────────────────────────────────────────────────────────────┘
```

## Agent Loop 上下文流

```
                     ┌──────────┐
                     │ 用户输入  │
                     └────┬─────┘
                          ▼
┌─────────────────────────────────────────────────────────┐
│ Agent.Run()                                              │
│                                                         │
│  ctxMgr.Add("user", input)                               │
│                                                         │
│  for step < maxIterations:                               │
│    ┌──────────────────────────────────────────────────┐ │
│    │ drainSteering()    ← 检查 Steer 注入的消息        │ │
│    │                                                    │ │
│    │ Reason(state):                                     │ │
│    │   callLLMForTool():                                │ │
│    │     ┌────────────────────────────────────────┐    │ │
│    │     │ autoCompactIfNeeded()  ← 五级预警检查   │    │ │
│    │     │   正常 → skip                          │    │ │
│    │     │   警告 → skip                          │    │ │
│    │     │   微压缩 → MicroCompact               │    │ │
│    │     │   压缩 → SessionMemory / LLM Summarize │    │ │
│    │     │   阻塞 → 拒绝新输入                    │    │ │
│    │     │                                        │    │ │
│    │     │ messages = Build(true)                 │    │ │
│    │     │ transformContext(messages) ← 20+ tool  │    │ │
│    │     │   results 时注入 keep-going 提示       │    │ │
│    │     │                                        │    │ │
│    │     │ ChatStream(messages, toolDefs) → LLM   │    │ │
│    │     │   每 token: AddTokens(1)               │    │ │
│    │     │   收到 usage: RecordUsage()            │    │ │
│    │     │   收到 finish_reason=length:           │    │ │
│    │     │     Tier1: maxTokens→64000             │    │ │
│    │     │     Tier2: disableThinking             │    │ │
│    │     │                                        │    │ │
│    │     │ AddAssistantToolCall()  → 存入 ctxMgr  │    │ │
│    │     └────────────────────────────────────────┘    │ │
│    │                                                    │ │
│    │ Execute → executeBatch(toolCalls)                   │ │
│    │   ReadTool: check FileStateCache first              │ │
│    │     hit → [File unchanged: ...] stub               │ │
│    │     miss → read + cache + MarkRead                  │ │
│    │   WriteTool/EditTool: check WasRead() first         │ │
│    │   AddToolResults → 存入 ctxMgr                     │ │
│    │   超限截断: max 2000 lines / 50KB                   │ │
│    │                                                    │ │
│    │ doomLoop检测: 3次相同tool call → 强制合成          │ │
│    │ diminishingReturns: 3次低输出 → 停止               │ │
│    └──────────────────────────────────────────────────┘ │
│                                                         │
│  loop end → forceSynthesize()                              │
│                                                         │
│  SummarizeIfNeeded():                                     │
│    NeedsSummarization? → 80% token budget                │
│    Try session memory first (free)                       │
│    Fallback: LLM summarizer                               │
│                                                         │
│  Session memory extraction:                               │
│    ShouldExtract? → 10K tokens / 5K growth + 3 tool calls│
│    RunAsync → LLM 提取 → write memory.md                  │
└─────────────────────────────────────────────────────────┘
```

## 压缩分层体系

```
┌──────────────────────────────────────────────────────────┐
│ 第〇层：Context Anchors (上下文锚点)                       │
│                                                          │
│ 触发：每次压缩前 (Compact / Summarize)                    │
│ 操作：标记关键消息，正则匹配识别：                          │
│   · 用户核心指令/任务描述                                  │
│   · 系统约束/API 版本要求                                  │
│   · 错误信息和修复方案                                     │
│   · 关键文件路径和行号                                     │
│ 效果：匹配的消息在压缩时优先保留，防止关键信息被误清除       │
│ 实现：bot/ctxmgr/anchor.go                                │
├──────────────────────────────────────────────────────────┤
│ 第一层：MicroCompact (微压缩)                              │
│                                                          │
│ 触发：estimatedTokens > tokenBudget / 2                  │
│ 操作：内容清除最近的 compactable tool result               │
│ 保留：最近 5 条结果                                       │
│ 标记：替换为 "[Old tool result cleared]"                  │
│ 零 LLM 成本                                              │
│                                                          │
│ compactableTools: read, bash, grep, glob,                │
│   web_search, web_fetch, edit, write                     │
├──────────────────────────────────────────────────────────┤
│ 第二层：Session Memory Compact (免费摘要)                  │
│                                                          │
│ 触发：SummarizeIfNeeded() 检查到 HasSubstance()           │
│ 操作：直接使用 session memory 文件内容作为摘要             │
│ 条件：len(content) > 100                                  │
│ 零 LLM 成本 — 复用已提取的笔记                            │
├──────────────────────────────────────────────────────────┤
│ 第三层：Full LLM Summarize (完整压缩)                      │
│                                                          │
│ 触发：NeedsSummarization() → 80% token budget             │
│ 流程：                                                    │
│   1. 只摘要 [compactBoundary, split] 区间的消息            │
│   2. 调用 summarizer (LLM) + 结构化模板                    │
│   3. 更新 m.summary                                       │
│   4. 推进 compactBoundary                                 │
│   5. 超 200 条旧消息时裁剪，snipped map 同步修复           │
│                                                          │
│ 摘要模板：                                                │
│   [Goal] → [Progress] → [Key Decisions] →                │
│   [Next Steps] → [Critical Context] → [Relevant Files]    │
│                                                          │
│ 保护规则：代码原文保留 / 错误信息逐字复制 / 文件路径保留    │
├──────────────────────────────────────────────────────────┤
│ 第三层½：摘要验证 (Summary Verification)                   │
│                                                          │
│ 触发：每次 LLM Summarize 完成后                            │
│ 流程：                                                    │
│   1. 检查摘要是否保留了代码片段（反引号块）                  │
│   2. 检查是否保留了错误信息（Error/panic/fatal 等关键词）    │
│   3. 检查是否保留了文件路径（/path/to/file:line 模式）      │
│   4. 检查摘要长度是否合理（不低于原始消息的 5%）             │
│   5. 验证失败 → 重新调用 summarizer（最多重试 1 次）        │
│                                                          │
│ 实现：bot/ctxmgr/summarize_verify.go                      │
├──────────────────────────────────────────────────────────┤
│ 熔断器 (预留)                                             │
│                                                          │
│ 连续 3 次压缩失败 → 停止自动压缩                           │
│ MaxConsecutiveFail = 3 (已定义，未接入)                    │
└──────────────────────────────────────────────────────────┘
```

## Token 追踪体系

```
┌──────────────────────────────────────────────────────────┐
│ Token 预算                                               │
│                                                          │
│ 默认上下文窗口: 64,000 tokens (DefaultTokenBudget)        │
│ 最大输出: 32,000 tokens (可升至 64,000)                   │
│                                                          │
│ 五级预警 (AutoCompactIfNeeded, bot/ctxmgr/auto_compact.go):│
│ ┌────────────────────┬──────────────────────────────────┐ │
│ │ Level              │ remaining token buffer           │ │
│ ├────────────────────┼──────────────────────────────────┤ │
│ │ Normal             │ > 20,000                         │ │
│ │ Warning            │ ≤ 20,000 (警告，不操作)           │ │
│ │ MicroCompact       │ ≤ 13,000 (触发微压缩)            │ │
│ │ Compact            │ ≤ 10,000 (触发完整压缩)           │ │
│ │ Blocking           │ ≤ 3,000 (拒绝新输入)             │ │
│ └────────────────────┴──────────────────────────────────┘ │
│                                                          │
│ AutoCompact 看门狗 (auto_compact.go):                      │
│   · 每次 Build() 前自动调用 AutoCompactIfNeeded()         │
│   · 根据 AccurateTokens() 与预算的差距判定级别             │
│   · Compact 级别：先尝试 Session Memory（免费），          │
│     失败再调用 LLM Summarizer                             │
│   · Blocking 级别：SetShouldBlock(true) 拒绝新输入        │
│   · 连续 3 次压缩失败 → 熔断，停止自动压缩                 │
│                                                          │
│ 双重估算:                                                │
│   TokenTracker (精确):                                    │
│     total = lastPromptTokens + lastCompTokens + newMsgEst │
│     RecordUsage() 在每次 API 流结束时调用                  │
│                                                          │
│   Heuristic (后备):                                       │
│     ASCII ~4 chars/token                                 │
│     CJK ~1.5 chars/token                                 │
│                                                          │
│   AccurateTokens():                                       │
│     tracker.Total() > 0 → 精确值                          │
│     else → estimatedTokens()                              │
└──────────────────────────────────────────────────────────┘
```

## FileStateCache 去重流程

```
ReadTool.Execute()
    │
    ▼
readTextCached(path, args)
    │
    ├─ 计算 offset, limit (default: 1, 2000)
    │
    ├─ GlobalFileCache.Get(path, offset, limit)
    │     │
    │     ├─ normalizePath() → cache key
    │     ├─ stat(mtime, size)
    │     └─ mtime/offset/limit 都匹配？
    │           │
    │           ├─ Yes → return stub:
    │           │   "[File unchanged: <name> — content matches
    │           │    previous read at offset=X limit=Y.]"
    │           │
    │           └─ No → read + cache
    │
    └─ readText(path) → 完整读取 + 行号格式化
          │
          └─ GlobalFileCache.Put(path, content, offset, limit, isPartial)
               │
               ├─ 更新 LRU order (移到最末)
               ├─ 驱逐最旧条目 (exceed 100 entries or 25MB)
               └─ isPartial 判定:
                   offset > 1 || content contains "\n\n[File has "

子 Agent 场景:
  主 agent GlobalFileCache (package-level)
       │
       ├─ sub-agent: 自动共享 (同一进程)
       └─ Clone() → 独立上下文场景
```

## Session Memory 系统

```
~/.nekocode/sessions/<session-id>/memory.md
    │
    ├─ 初始化: 写入 memory_template.md
    │
    ├─ 提取触发 (ShouldExtract):
    │   首次: tokenCount ≥ 10,000
    │   后续: token 增长 ≥ 5,000 AND (toolCalls ≥ 3 OR lastTurn无工具调用)
    │
    ├─ 异步提取 (RunAsync):
    │   1. 获取当前 memory.md 内容
    │   2. 获取当前消息历史 (Build(false), 不含工具定义)
    │   3. 发送给 LLM → 更新 memory.md
    │   4. 超时保护 (goroutine 不阻塞主循环)
    │
    ├─ 用作免费摘要:
    │   SummarizeIfNeeded(): HasSubstance() → SummarizeWithSessionMemory()
    │   AutoCompactIfNeeded(): sessionMemoryProvider()
    │   ForceFreshStart(): 优先使用 memory content
    │
    └─ 模板结构:
        # Session Title
        # Current State ← 当前进度
        # Task Specification ← 用户需求
        # Files and Functions ← 重要文件
        # Workflow ← 常见操作
        # Errors & Corrections ← 错误修复记录
        # Learnings ← 经验教训
        # Key Results ← 输出结果
        # Worklog ← 操作记录
```

## Project Context 预加载

```
LoadProjectContext(cwd)
    │
    ├─ 1. ~/.nekocode/NEKOCODE.md     (用户全局)
    │
    ├─ 2. cwd → root 向上遍历:
    │     每个目录查找:
    │       {dir}/NEKOCODE.md          (项目根)
    │       {dir}/.nekocode/NEKOCODE.md (隐藏项目配置)
    │       {dir}/.nekocode/rules/*.md (条件规则, 按名排序)
    │
    ├─ 3. 去重 (EvalSymlinks)
    │
    └─ 4. buildContext(unique files)
         │
         ├─ 40K chars 总预算
         ├─ @include 递归 (max depth=3)
         │   支持: @./relative, @../up, @~/home, @/absolute
         │   过滤: looksLikePath(含/或已知扩展名)
         │   循环保护: processed map
         │
         └─ 输出: <project-context>\n...\n</project-context>
              注入为独立 system message
```

## Snip 系统

```
模型调用 snip(startIdx, endIdx)
    │
    ▼
ctxMgr.Snip(startIdx, endIdx)
    │
    ├─ 范围校验: [0, len(messages)-1]
    ├─ 标记 snipped map[startIdx..endIdx] = true
    │
    └─ Build() 中的效果:
         origIdx 转换: origBase + kept_offset
         snipped[origIdx] → skip 该消息

索引稳定性保证:
  [id:N] 标签注入在 user messages 上
  compactBoundary 保留旧消息 → 索引不偏移
  summarizeInternal 裁剪时同步修复 snipped map
```

## 关键配置参数

| 参数 | 默认值 | 位置 |
|------|--------|------|
| 默认上下文窗口 | 64,000 | `ctxmgr/manager.go` |
| 消息窗口大小 | 20 | `ctxmgr/manager.go` |
| Warning 阈值 | ≤ 20,000 | `ctxmgr/auto_compact.go` |
| MicroCompact 阈值 | ≤ 13,000 | `ctxmgr/auto_compact.go` |
| Compact 阈值 | ≤ 10,000 | `ctxmgr/auto_compact.go` |
| Blocking 阈值 | ≤ 3,000 | `ctxmgr/auto_compact.go` |
| FileStateCache 条目 | 100 | `tools/file_cache.go` |
| FileStateCache 大小 | 25 MB | `tools/file_cache.go` |
| Project Context 上限 | 40K chars | `context/project.go` |
| Include 最大深度 | 3 | `context/project.go` |
| 压缩后保留消息数 | keep = windowSize/2 | `ctxmgr/summarize.go` |
| boundary 前最大保留 | 200 | `ctxmgr/summarize.go` |
| 后压缩文件预算 | 20K chars | `ctxmgr/summarize.go` |
| 后压缩文件数 | 5 | `ctxmgr/summarize.go` |
| Session Memory 首次提取 | 10,000 tokens | `session/memory.go` |
| Session Memory 增量提取 | 5,000 tokens | `session/memory.go` |
| 工具输出最大行数 | 2,000 | `tools/truncate.go` |
| 工具输出最大字节 | 50KB | `tools/truncate.go` |
| Snip 保留最近 compactable | 5 条 | `ctxmgr/compact.go` |
| 最大输出 tokens | 32,000 (64,000 上限) | `agent/reasoner.go` |
| 摘要验证最大重试 | 1 次 | `ctxmgr/summarize_verify.go` |

## 文件索引

| 文件 | 职责 |
|------|------|
| `bot/bot.go` | Agent 初始化、上下文配置、summarize/session-memory 调度 |
| `bot/ctxmgr/manager.go` | 消息存储、Build() 组装、compact boundary 管理、token 预算 |
| `bot/ctxmgr/token.go` | 语言感知 token 估算 (ASCII/CJK) |
| `bot/ctxmgr/storage.go` | Add/AddToolResult/Clear/FreshStart + BoltDB 持久化 |
| `bot/ctxmgr/compact.go` | MicroCompact — 旧工具结果清除 |
| `bot/ctxmgr/anchor.go` | 上下文锚点 — 压缩时保留关键用户指令和系统约束 |
| `bot/ctxmgr/auto_compact.go` | AutoCompactIfNeeded、五级预警、TokenTracker、CompactLevel、熔断器 |
| `bot/ctxmgr/summarize.go` | Summarize/SummarizeWithSessionMemory/BuildPrompt/PostCompactFiles |
| `bot/ctxmgr/summarize_verify.go` | 摘要验证 — LLM 生成的摘要二次校验后写入 |
| `bot/context/project.go` | NEKOCODE.md 发现与加载、@include 递归 |
| `bot/tools/file_cache.go` | FileStateCache — LRU + mtime 去重、Get/Put/GetContent/Merge/Clone |
| `bot/tools/guard.go` | 安全门控 — 敏感工具操作确认提示 |
| `bot/tools/tool_read.go` | ReadTool — 读取前查缓存命中返回 stub |
| `bot/tools/executor.go` | 工具执行调度、读-写保护、截断 |
| `bot/tools/truncate.go` | 工具输出截断 (2000行/50KB) |
| `bot/session/memory.go` | Session Memory 管理、异步 LLM 提取 |
| `bot/session/memory_template.md` | Session Memory 模板 |
| `bot/agent/reasoner.go` | callLLMForTool、forceSynthesize、length 升级、usage 记录 |
| `bot/agent/subagent/engine.go` | 子 Agent 独立上下文循环、auto-compact |
| `bot/skill/skill.go` | Skill 注册表 + Workflow 类型 + 运行时引擎 |
| `bot/skill/inject.go` | 技能工作流注入 system prompt |
| `bot/prompt/system.md` | 猫娘角色 system prompt |

## 数据流全景

```
                          ┌─────────────────────┐
                          │   ~/.nekocode/       │
                          │   config.json        │
                          │   (provider, apiKey, │
                          │    model, budget)    │
                          └──────┬──────────────┘
                                 │ Config
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Bot.New()                                                          │
│                                                                     │
│  1. LoadConfig() → provider / model / tokenBudget / thinkingBudget   │
│  2. ctxMgr ← embed systemPrompt + env reminder + project context     │
│  3. llmClient ← provider switch                                      │
│  4. summarizer ← LLM closure (ctxMgr.BuildPrompt + Chat)             │
│  5. toolRegistry ← RegisterDefaults (read/write/edit/bash/...)       │
│  6. GlobalFileCache ← NewFileStateCache()                            │
│  7. sessMem ← session.New(sessionID, template)                       │
│  8. agent ← agent.New(ctxMgr, llmClient, toolRegistry)               │
│  9. subEngine ← subagent.NewEngine(cloneLLM, toolRegistry)           │
│ 10. Wire callbacks (snip, circuit breaker, context transform, etc.)   │
└────────────────────────────────┬────────────────────────────────────┘
                                 │
                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Bot.RunAgent(input, onStep)                                         │
│                                                                     │
│  agent.Run(input, onStep)                                            │
│    │                                                                │
│    ├─ ctxMgr.Add("user", input)                                     │
│    ├─ Loop (max 15 iterations):                                     │
│    │   autoCompact → Build → transform → ChatStream → Execute       │
│    │   doomLoop / diminishingReturns checks                         │
│    ├─ forceSynthesize() (if exhausted)                              │
│    │                                                                │
│    └─ Return result                                                   │
│                                                                     │
│  SummarizeIfNeeded()                                                 │
│    └─ SM content → LLM summarizer (fallback)                        │
│                                                                     │
│  ShouldExtract() → RunAsync (goroutine)                              │
│    └─ LLM: update session memory.md                                  │
└─────────────────────────────────────────────────────────────────────┘
```
