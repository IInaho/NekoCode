# PrimusBot 开发路线

按优先级排列，每项可独立实施验证。✅ = 已完成，🟡 = 部分完成。

---

## P0 — 核心功能

### 1. 精确编辑工具 (EditTool) ✅
- 精确替换文件中首次出现的字符串
- 失败时返回带行号的文件内容 + diff

### 2. 内容搜索工具 (GrepTool) ✅
- 基于 ripgrep 的内容搜索，支持 regex/glob/context

### 3. Diff 展示 ✅
- EditTool 执行后返回 +- 行级对比

### 4. 结构化内容块 (ContentBlock) ✅
- 工具调用聚合在单卡片，暖金色 NormalBorder
- 💭 思考块独立渲染，多行自动对齐
- `ctrl+e` 翻转 tool block Collapsed

### 5. TUI-Bot 解耦 ✅
- `BotInterface` 接口（14 个方法）
- `bot/types` 共享类型包
- Bot 通过接口暴露，TUI 零耦合

### 6. 项目感知上下文 🟡
- ✅ `buildDirectoryTree`：并发行数统计 + 目录树注入 system prompt
- ❌ CLAUDE.md / AGENTS.md 读取
- ❌ .gitignore 排除

### 7. Skills / Extensions 系统 🟡
- ✅ Extension 接口：Tools() + Commands()
- ✅ 命令注册/分发机制
- ❌ 触发条件 / 动态激活

### 8. Web Search / Fetch ✅
- `web_search`：Exa AI MCP 优先（free tier）→ Bing HTML 降级，numResults 可配
- `web_fetch`：HTML→Markdown + DNS 安全校验
- ANSI escape 序列清理

---

## P1 — 架构增强

### 9. Provider 合并 ✅
- OpenAI / GLM / DeepSeek → `OpenAICompatible`
- Anthropic SSE 流式正确解析 content_block_start/delta

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
- `StripAnsi` 导出函数，bash/webfetch/filesystem 全面覆盖

### 14. 并行工具执行 ✅
- executor `ExecuteBatch` 根据 ExecutionMode 串行/并行
- worker pool 上限 10

### 15. 处理阶段独立模块 ✅
- `phase.go`：Ready/Thinking/Reasoning/Running 状态机

### 16. Scrollbar 独立组件 ✅
- 独立 `Scrollbar` 组件，与 Messages 并列渲染

### 16a. BTW 中断机制 ✅
- Processing 中直接打字 + Enter 注入新消息并打断当前 LLM 调用
- Esc 纯 Abort，返回"已中断"
- 原子 ctx 替换（ctxMu 保护并发），UI 实时反馈

### 16b. 指数退避重试 ✅
- `bot/agent/retry.go`：LLM 调用失败自动重试
- 0.5s→1s→2s→4s→8s（最多 4 次），区分可重试/不可重试错误

### 16c. 模块重组 ✅
- ctxmgr 移入 `bot/ctxmgr/`（仅 bot 使用，依赖方向更清晰）

### 17. 子 Agent 并行
- 复杂任务拆分为独立子任务，并行执行，结果汇总

### 18. 后台任务 + 进度
- 长运行命令流式输出，不阻塞主 Agent 循环

### 19. Checkpoint / Undo
- 每次工具写入前自动保存快照
- `/undo` 命令回滚

### 20. 任务列表 (Todo tracking)
- 复杂请求自动生成 Todo，TUI 实时显示进度

---

## P2 — 生态与体验

### 21. MCP 协议支持
- MCP client，连接外部 tool server

### 22. Session 管理
- 对话存档/恢复，支持分支对话

### 23. Plan 模式
- 复杂改动先出方案文本，用户审批后执行

### 24. 凭证管理
- API key 安全存储，多 profile 切换

### 25. 自动化测试
- Agent 行为回归测试（mock LLM 响应）
- 工具执行单元测试（mock 文件系统/shell）
