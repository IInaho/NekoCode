# PrimusBot 开发路线

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
- `BotInterface` 接口（17 个方法）
- Phase 类型和 Confirm 类型统一定义在 `bot/tools/`
- Bot 通过接口暴露，TUI 零耦合

### 6. 项目感知上下文 🟡
- ✅ working directory 作为 system-reminder 注入
- ❌ CLAUDE.md / AGENTS.md 读取
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
- 10 section Markdown 文件：`~/.primusbot/sessions/<id>/memory.md`
- `/new` 命令用 session memory 做免费摘要（不调 API）

### 12. Snip 工具 ✅
- 模型通过 `snip` 工具主动移除旧消息范围
- `[id:N]` 标签仅在 API 侧注入，用户不可见
- 被 snipped 的消息在后续 Build() 中过滤

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

---

## P2 — 生态与体验

### 28. 后台任务 + 进度
- 长运行命令流式输出，不阻塞主 Agent 循环

### 29. Checkpoint / Undo
- 每次工具写入前自动保存快照
- `/undo` 命令回滚

### 30. MCP 协议支持
- MCP client，连接外部 tool server

### 31. Session 管理
- 对话存档/恢复，支持分支对话

### 32. Plan 模式
- 复杂改动先出方案文本，用户审批后执行

### 33. 凭证管理
- API key 安全存储，多 profile 切换

### 34. 自动化测试
- Agent 行为回归测试（mock LLM 响应）
- 工具执行单元测试（mock 文件系统/shell）
