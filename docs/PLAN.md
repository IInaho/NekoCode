# NekoCode 开发路线

> **本文档职责**: 追踪已完成和待办的功能项。记录开发里程碑、实施状态（✅/🟡）。每项简要描述功能目标，不展开设计或架构细节（细节属于 DESIGN.md / ARCHITECTURE.md）。更新时请保持此边界。

按优先级排列，每项可独立实施验证。✅ = 已完成，🟡 = 部分完成。

---

## P0 — 核心功能

### 1. 精确编辑工具 (EditTool) ✅
- 精确替换文件中首次出现的字符串
- 失败时返回带行号的文件内容 + diff
- diff 嵌入工具块，`[+]`/`[-]` 折叠展开

### 2. 内容搜索工具 (GrepTool) ✅
- 基于 ripgrep 的内容搜索，支持 regex/glob/context

### 3. Diff 展示 ✅
- EditTool diff 高亮着色（+绿/-红），嵌入工具卡片

### 4. 结构化内容块 (ContentBlock) ✅
- block/ 子包：BlockType 枚举 + 渲染 + FilterFinalBlocks
- 工具组折叠：同名单行工具收为一行 `[+]` 展开
- Ctrl+E 切换组和 edit 块的折叠状态
- `BuildToolGroups` 在 streaming 和 message 两侧共享

### 5. TUI-Bot 解耦 ✅
- `BotInterface` 接口（18 个方法）
- Phase 类型和 Confirm 类型统一定义在 `bot/tools/`
- Bot 通过接口暴露，TUI 零耦合

### 6. 项目感知上下文 🟡
- ✅ working directory 作为 system-reminder 注入
- ✅ NEKOCODE.md 自动发现 + @include 递归加载（~/.nekocode/ → 项目根/ → .nekocode/ → rules/）
- ❌ .gitignore 排除

### 7. Web Search / Fetch ✅
- `web_search`：Exa MCP（JSON-RPC over SSE）
- `web_fetch`：HTML→Markdown + DNS 校验 + 内网 IP 拒绝

---

## P1 — 架构增强

### 8. Provider 合并 ✅
- OpenAI / GLM / DeepSeek → `OpenAICompatible`
- Anthropic SSE 流式解析 content_block_start/delta
- `disableThinking` 接入 API 请求体

### 9. 上下文窗口优化 ✅
- ctxmgr 拆分为 5 文件：manager / compact / storage / token / summarize
- 语言感知 token 估算（CJK ~1.5/token, ASCII ~4/token）
- Build() 保护 tool_calls/tool_result 配对
- 结构化摘要：Goal/Progress/Key Decisions/Next Steps/Critical Context/Relevant Files，增量更新

### 10. 微压缩 ✅
- 清除旧 compactable 工具结果（read、bash、grep、glob、web_*、edit、write）
- 保留最近 5 个，替换为 `[Old tool result cleared]`
- **仅在 token > 50% 预算时激活**——探索期不丢上下文
- 状态栏显示 `🧹N` 累计计数

### 11. Session Memory ✅
- 异步提取：goroutine 方式，10k+ token 开始，+5k token + 3 tool call 再触发
- 10 section Markdown 文件：`~/.nekocode/sessions/<id>/memory.md`
- `/new` 命令用 session memory 做免费摘要（不调 API）

### 12. Snip 工具 ✅ → 已移除
- 模型通过 `snip` 工具主动移除旧消息范围
- `[id:N]` 标签仅在 API 侧注入，用户不可见
- 被 snipped 的消息在后续 Build() 中过滤
- **已移除**：技能上下文改用 `RemoveMessages()` 直接清除，snip 复杂度过高且收益有限

### 13. `/new` 命令 ✅
- 开始新对话，保留上一任务摘要
- 优先用 session memory，fallback 到 API summarizer

### 14. 共享 HTTP 客户端 ✅
- `SharedHTTPClient` + `SharedHTTPClientTimeout`
- Transport 连接池复用

### 15. 确认框 ✅
- 卡片式布局：Tool / File / Level + Prompt

### 16. ANSI 清理 ✅
- `StripAnsi` 在 util.go，bash/webfetch 全面覆盖

### 17. 并行工具执行 ✅
- Executor `ExecuteBatch`：partition + parallel/serial + danger check
- worker pool 上限 10
- subagent 共享同一个 Executor

### 18. 处理阶段 ✅
- `tools.Phase*` 常量 agent 和 TUI 统一引用

### 19. Scrollbar 独立组件 ✅

### 20. BTW 中断机制 ✅
- Processing 中直接打字 + Enter 注入新消息并打断当前 LLM 调用
- Esc 纯 Abort，返回 "Interrupted"
- `replaceCtx()` 使用 `parentCtx` 保持取消链

### 21. 指数退避重试 ✅
- `bot/agent/retry.go`：LLM 调用失败自动重试
- 0.5s→1s→2s→4s→8s（最多 4 次）
- token 统计防重复累加

### 22. 模块重组 ✅
- 删除 `bot/types/`（类型移入 `bot/tools/`）
- 删除 `bot/extensions/`（YAGNI，无实现）
- 拆分 `bot/config.go` → `config.go` + `commands.go`
- 拆分 `bot/tools/tool.go` → `tool.go` + `util.go` + `descriptor.go` + `confirm.go` + `executor.go`
- `Executor` 从 `bot/agent/` 移入 `bot/tools/`（subagent 共享）
- TUI 拆分为 block/message/processing 子包

### 23. 子 Agent 系统 ✅
- 5 种内置类型：executor / verify / explore / plan / decompose
- 独立 Engine，共享 `tools.Executor`
- disableThinking=true（API 级 + prompt 级）
- 上下文隔离（每次创建新 ctxmgr）

### 24. 任务列表 (Todo tracking) ✅
- `todo_write` 工具：记录任务状态
- TUI 实时显示进度，注入 agent 上下文

### 25. 代码质量 ✅
- 死代码清理（ParseCall/unquote、extensions package、NeedFreshStart、SystemPrompt）
- 路径穿越防护（validatePath）
- 工具描述和系统提示词全英文化（人设部分保留中文）
- Go vet 零警告

### 26. 输出噪声过滤 ✅
- `isEmptyOrNoise()` 过滤纯空白、纯点号、纯符号行
- 全部噪声时 output 块不渲染
- LLM 流式产出的空白不再显示为空白 output 区块

### 27. 文档更新 ✅
- ARCHITECTURE.md / DESIGN.md / PLAN.md 反映当前项目状态

### 28. 幻觉防治体系 ✅
基于主流方案调研实施的 14 项防幻觉改造：
- System Prompt 增强（禁止生成 URL、忠实报告、Prompt Injection 检测、当前状态权威）+ 日期注入
- 末日循环检测（4 次相同调用 → forceSynthesize）
- 工具输出预算截断（2000行/50KB）
- "先读后改"运行时强制（Edit/Write 前校验 Read 记录）
- web_search/fetch 来源引用强制
- verify agent 格式强化（Command block 强制 + 自检清单）
- 记忆漂移防护（模板警告）
- 二进制文件检测（null 字节 + 不可打印字符比例）
- 文件未找到智能建议（Levenshtein 相似度匹配）
- 摘要压缩保真度增强（Critical Preservation Rules）
- Edit 3 轮渐进模糊匹配（精确 → Rstrip → Strip）
- bash 命令智能分级（只读命令自动 Safe）
- 危险等级标签去歧义（safe/modify/danger/blocked）
- 确认框展示具体命令/路径
- 思考模式管理：Anthropic 默认 adaptive，DeepSeek 默认关 thinking，两级 finish_reason=length 升级
- reasoning token 纳入统计
- edit 工具组内联展开（▍ path + diff 一次可见，无需二次折叠）
- 跨目录 edit/write 允许（确认框管控）
- bash 复杂命令显示截断（只展示首行 + …）
- 工具组展开子项缩进 2 格
- Search/Fetch 断路器（≥4 次搜索 0 次抓取 → 强制停止）
- ContextTransform 工具结果监控（>20 条结果提示检查子任务）
- Task 子 agent 结果内联显示（输出附加到 task 工具块，支持折叠展开）

### 29. Skill 系统 ✅
- YAML 定义技能工作流（Ref / Prompt / Tool / Invoke 步骤类型）
- `.nekocode/skills/` 目录自动发现
- 工作流注入 system prompt
- `/<skill-name>` 斜杠命令触发
- `/skill` 工具供 Agent 调用

### 30. 上下文锚点 ✅
- 压缩前自动标记关键用户指令和系统约束
- 正则匹配保留 API 版本要求等关键信息

### 31. 摘要验证 ✅
- LLM 生成的摘要经二次 LLM 校验
- 检查代码片段、错误信息、文件路径等关键内容是否丢失

### 32. 文件缓存 ✅
- `GlobalFileCache`：LRU 驱逐（100 条目 / 25MB 上限）
- mtime + offset + limit 精确去重
- 跨子 Agent 共享同一缓存实例

### 33. 五级预警自动压缩 ✅
- Normal → Warning → MicroCompact → Compact → Blocking
- `AutoCompactIfNeeded()` 每次 Build() 前自动判定

### 34. 对话历史存取 ✅
- 消息存取：Add/AddToolResult/Clear/FreshStart
- `ctxmgr/storage.go` 管理存取（内存存储，非持久化）

### 35. NEKOCODE.md 项目上下文 ✅
- 多层级目录发现（~/.nekocode/ → 项目根/ → .nekocode/ → rules/）
- @include 递归引用（最大深度 3）
- 40K 字符预算

### 36. 子包拆分：config + command ✅
- `bot/config.go` → `bot/config/config.go`：独立 config 子包
- `bot/commands.go` → `bot/command/parser.go`：独立 command 子包，Callbacks 模式解耦
- `RegisterDefaults()` 集中注册 help/clear/stats/summarize/new/config 命令

### 37. 工具实现重组：tools/builtin ✅
- 工具实现从 `bot/tools/tool_*.go` 移入 `bot/tools/builtin/tool_*.go`
- 新增 `bot/tools/builtin/register.go`：`RegisterAll()` 统一注册全部 12 个内置工具
- `bot/tools/` 保留核心抽象（接口、Registry、Executor、Phase、Confirm 等）

### 38. TUI handlers 合并 ✅
- `tui/handlers_spin.go` + `tui/handlers_done.go` + `tui/handlers_keys.go` → `tui/handlers.go`
- `tui/phase.go` 删除，Phase 常量移入 `tui/model.go`

### 39. Markdown 渲染移动 ✅
- `tui/styles/markdown.go` → `tui/components/message/markdown.go`
- 按宽度缓存的 glamour renderer，Warmup() 预创建 40-160 字符宽度的渲染器

### 40. Bundled Skills ✅
- 新增 `bot/skill/bundled/` 子包
- `go:embed` 加载 `bundled/meta/SKILL.md`，编译进二进制
- Bundled skills 优先注册，优先级高于文件系统技能

### 41. 文档更新 ✅
- ARCHITECTURE.md / DESIGN.md / CONTEXT_DESIGH.md / PLAN.md 反映当前项目状态

### 42. 分层 System Prompt ✅

### 42. 分层 System Prompt ✅
- `system_claude.md`：think-first + plan-before-code（Claude/GPT）
- `system_deepseek.md`：act-first + anti-overthinking（DeepSeek/GLM）
- `bot.go` 按 `cfg.Provider` 自动选择

### 43. 工具描述精准化 ✅
- bash/edit/write/task 四大核心工具描述大幅扩展
- 包含使用时机、反模式、常见错误、具体示例

### 44. TodoWrite 一等公民 ✅
- 两套 prompt 均新增 `# Task Tracking` 章节
- 3+ 步骤任务必须先用 todo_write，实时更新状态

### 45. Anthropic Prompt Caching ✅
- `llm/anthropic.go`：3 个 `cache_control: {type: "ephemeral"}` 断点
- system prompt / last tool def / 倒数第 4 条消息 标记缓存
- `anthropic-beta: prompt-caching-2024-07-31` 请求头
- `StreamUsage` 追踪 `cache_read/creation_input_tokens`

### 46. DeepSeek Prefix Cache 优化 ✅
- `Build()` 三阶段重排：static → stable(messages) → dynamic(todo/summary/skill)
- micro-compaction 保留数按 token budget 分级（5/8/12）
- 最大化服务端 automatic prefix caching 命中率

### 47. Plan Mode ✅
- `/plan <task>` 命令进入只读探索模式
- Executor planMode 阻断 write/edit/destructive-bash
- 一个回合后自动退出，用户可直接继续执行

### 48. Auto-Review 自动验证 ✅
- 文件修改后 → 单次注入 build+test 提示
- 修 bug 未复现 → 注入复现+验证提示
- `filesModified`（write/edit 触发）、`didReproduce`（bash 失败触发）、`verifyInjected`（防重复）三个字段

### 49. Bug 分析反幻觉 ✅
- 两套 prompt 新增 `# Debugging` 6 阶段方法论 + Anti-patterns 清单
- 删除了 `isModifyingCommand` / `isVerifyCommand` / `isReproCommand` 三个不可靠的字符串匹配函数
- `didReproduce` 基于实际结果判断（exit≠0 或输出含 error/panic），而非命令名匹配

### 50. 智能截断 ✅
- head(40) + tail(20) 双保留，错误时扩展 tail 到 60+
- 重复行折叠（连续 8 行相同 → `[repeated N times]`）
- `isErrorLine` 检测 20+ 错误模式

### 51. 任务列表 UI 主题化 ✅
- emoji 替换为 `·` `▸` `✓` Unicode 字符
- `renderTodos()` 状态着色（Subtle/Teal/DiffGreen）
- 全部完成显示绿色摘要而非隐藏

### 52. 长任务友好 ✅
- `maxIterations` 15→30，`config.json` 可配
- `doomLoopWindow` 3→4，`lowOutputThreshold` 3→5
- `ContextTransform` 阈值 20→40 tool results

### 53. Edit 模糊匹配修复 ✅
- `findWithFuzzy` 返回 `(start, end, pass)` 三值
- `mapNormPosToOrig` 提取为独立函数
- 修复 fuzzy match 时 `idx+len(oldStr)` 越界 panic

### 54. 子 Agent 安全分类器 ✅
- `bot/agent/subagent/classify.go`：pattern-matching 零成本安全审核
- `bot/agent/subagent/result.go`：结构化 Result 类型 + XML 序列化 + Status/Classification
- 子 agent 结果注入 SECURITY WARNING 前缀，不阻断信息流

### 55. LLM Factory + Clone 一致性 ✅
- `llm/factory.go`：NewClient / Clone 统一 provider 感知工厂
- Clone 传递 thinkingBudget，消除与 NewClient 的行为不一致
- Anthropic 构造函数统一接受 baseURL 参数

### 56. 流式 HTTP 客户端超时 ✅
- `SharedHTTPStreamClient`（10min）：防止服务端 hang 导致 goroutine 泄漏
- ChatStream 改用独立超时客户端，同步 Chat 保持 120s

### 57. 智能重试判定 ✅
- HTTP 状态码提取（正则匹配两种错误格式）替代字符串关键字匹配
- 408/429/5xx → 可重试，其余 4xx → 不可重试
- 网络层错误关键字兜底

### 58. Thinking 方法跨 Provider 互译 ✅
- Anthropic `SetReasoningEffort`：high/max → thinking enabled + budget tokens；不再是空函数
- OpenAICompatible `SetThinkingBudget`：token 数量 → reasoning_effort 映射；不再是简单 on/off
- 调用方可无视 provider 使用任一方法

### 59. Bot 生命周期管理 ✅
- `bot/lifecycle.go`：SummarizeIfNeeded() / ForceSummarize() / ForceFreshStart()
- `bot/commands.go`：RegisterDefaults() 集中注册内置命令
- ContextReport：结构化诊断报告（消息/摘要/tool def token 用量）

---

## P2 — 生态与体验

### 60. 后台任务 + 进度
- 长运行命令流式输出，不阻塞主 Agent 循环

### 61. Checkpoint / Undo
- 每次工具写入前自动保存快照
- `/undo` 命令回滚

### 62. MCP 协议支持
- MCP client，连接外部 tool server

### 63. Session 管理
- 对话存档/恢复，支持分支对话

### 64. 凭证管理
- API key 安全存储，多 profile 切换

### 65. 自动化测试
- Agent 行为回归测试（mock LLM 响应）
- 工具执行单元测试（mock 文件系统/shell）
