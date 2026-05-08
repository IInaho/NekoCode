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
- block/ 子包：BlockType 枚举 + 5 种渲染 + FilterFinalBlocks
- 仅 edit 工具显示 `[+]`/`[-]` 折叠
- 所有块统一 2 字符缩进
- `ctrl+e` 翻转 edit 工具块 Collapsed

### 5. TUI-Bot 解耦 ✅
- `BotInterface` 接口（17 个方法）
- `bot/types` 共享类型包（Phase 常量统一定义）
- Bot 通过接口暴露，TUI 零耦合

### 6. 项目感知上下文 🟡
- ✅ working directory 作为 system-reminder 注入
- ❌ CLAUDE.md / AGENTS.md 读取
- ❌ .gitignore 排除

### 7. Skills / Extensions 系统 ✅
- Extension 接口：Tools() + Commands()
- 命令注册/分发机制

### 8. Web Search / Fetch ✅
- `web_search`：Exa MCP 优先 → Bing HTML 降级
- `web_fetch`：HTML→Markdown + DNS 安全校验
- Exa API key 通过 Header 传递（不在 URL query）

---

## P1 — 架构增强

### 9. Provider 合并 ✅
- OpenAI / GLM / DeepSeek → `OpenAICompatible`
- Anthropic SSE 流式解析 content_block_start/delta
- `disableThinking` 接入 API 请求体（子 agent 关闭 thinking）

### 10. 上下文窗口优化 ✅
- ctxmgr 拆分为 4 文件：manager / storage / token / summarize
- 语言感知 token 估算（CJK ~1.5/token, ASCII ~4/token）
- Build() 保护 tool_calls/tool_result 配对
- 结构化摘要：目标/进展/关键决策/下一步/关键上下文/相关文件，增量更新

### 11. 共享 HTTP 客户端 ✅
- `SharedHTTPClient` + `SharedHTTPClientTimeout`
- 底层 Transport 连接池复用

### 12. 确认框重构 ✅
- 卡片式布局：Tool / File / Level + Prompt

### 13. ANSI 转义序列清理 ✅
- `StripAnsi` 导出函数，bash/webfetch 全面覆盖

### 14. 并行工具执行 ✅
- executor `ExecuteBatch` 根据 ExecutionMode 串行/并行
- worker pool 上限 10
- 并行前 ctx 取消检查

### 15. 处理阶段独立模块 ✅
- `phase.go`：Phase 常量在 `bot/types` 统一定义，agent 和 TUI 两边引用

### 16. Scrollbar 独立组件 ✅

### 16a. BTW 中断机制 ✅
- Processing 中直接打字 + Enter 注入新消息并打断当前 LLM 调用
- Esc 纯 Abort，返回"已中断"
- `replaceCtx()` 使用 `parentCtx` 保持取消链

### 16b. 指数退避重试 ✅
- `bot/agent/retry.go`：LLM 调用失败自动重试
- 0.5s→1s→2s→4s→8s（最多 4 次）
- token 统计防重复累加

### 16c. 模块重组 ✅
- ctxmgr 移入 `bot/ctxmgr/`，Phase 常量移入 `bot/types/`
- TUI 拆分为 block/message/processing 子包
- 目录结构精简（subagent 合并 agents.go，config+command 合并）

### 17. 子 Agent 系统 ✅
- 5 种内置子 agent：executor / verify / explore / plan / decompose
- 独立 Engine（不依赖主 agent 包，避免循环依赖）
- disableThinking=true（API 级 + prompt 级）
- 上下文隔离（每次创建新 ctxmgr）
- Run 循环 ctx 取消检查

### 18. 任务列表 (Todo tracking) ✅
- `todo_write` 工具：记录任务状态
- TUI 实时显示进度，注入 agent 上下文

### 19. 代码质量 ✅
- 全项目死代码清理（BlockStream、channel streaming、duplicate fmtTokens 等）
- 路径穿越防护（validatePath）
- 不安全类型断言修复
- Go vet 零警告

### 20. 文档更新 ✅
- ARCHITECTURE.md / DESIGN.md / PLAN.md 反映当前项目状态

---

## P2 — 生态与体验

### 21. 后台任务 + 进度
- 长运行命令流式输出，不阻塞主 Agent 循环

### 22. Checkpoint / Undo
- 每次工具写入前自动保存快照
- `/undo` 命令回滚

### 23. MCP 协议支持
- MCP client，连接外部 tool server

### 24. Session 管理
- 对话存档/恢复，支持分支对话

### 25. Plan 模式
- 复杂改动先出方案文本，用户审批后执行

### 26. 凭证管理
- API key 安全存储，多 profile 切换

### 27. 自动化测试
- Agent 行为回归测试（mock LLM 响应）
- 工具执行单元测试（mock 文件系统/shell）
