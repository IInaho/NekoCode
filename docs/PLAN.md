# PrimusBot 开发路线

按优先级排列，每项可独立实施验证。

---

## P0 — 核心功能

### 1. 精确编辑工具 (EditTool) ✅ 已完成
- 替代当前 `filesystem.write` 的覆盖写入
- 接口：`Edit(old_string, new_string, path)` — 精确替换文件中首次出现的字符串
- 失败时返回友好的 diff 对比

### 2. 内容搜索工具 (GrepTool) ✅ 已完成
- 基于 ripgrep 的内容搜索，非文件名匹配
- 接口：`Grep(pattern, path?, glob?)` — 返回匹配行 + 行号
- 支持 `-A/-B/-C` 上下文行数

### 3. 项目感知上下文
- 启动时自动加载项目信息注入 system prompt：
  - 读取 `CLAUDE.md` / `AGENTS.md`（项目规则、编码规范）
  - 读取 `.gitignore`（排除文件）
  - 生成目录树摘要
- `ctxmgr.Build()` 时自动附加项目上下文

### 4. Diff 展示 ✅ 已完成
- `EditTool` 执行后返回 old→new 的行级对比
- TUI 在 Assistant 消息中渲染 diff（+ 绿色，- 红色）

### 5. Skills 系统
- 可动态注册的能力模块
- 每个 skill 定义：名称、描述、触发条件、关联工具集
- 示例 skill：`code-review` 触发时自动加载 GrepTool + EditTool + 特定 system prompt
- 用户通过 `# <skill名>` 或自然语言触发

---

## P1 — 架构增强

### 6. 子 Agent 并行
- 复杂任务拆分为独立子任务，并行执行，结果汇总
- 每个子 Agent 共享工具注册表但独立上下文

### 7. 后台任务 + 进度
- 长运行命令（`go build`、`npm install`）流式输出
- TUI 显示后台任务进度条，不阻塞主 Agent 循环

### 8. Checkpoint / Undo
- 每次工具写入前自动保存快照（文件内容 + 时间戳）
- `/undo` 命令回滚到最后一次 checkpoint
- 历史记录可追溯、可比较

### 9. 任务列表 (Todo tracking)
- 复杂请求自动生成 Todo 列表
- 每完成一项勾掉，TUI 实时显示进度
- 用户可手动添加/调整优先级

### 10. LSP 集成
- 连接 language server 获取诊断、跳转、引用
- GrepTool 结果可关联到符号定义

---

## P2 — 生态与体验

### 11. MCP 协议支持
- 实现 MCP client，连接外部 tool server
- 社区 MCP server 开箱即用

### 12. Web Search / Fetch
- 搜索最新文档、错误信息
- 读取 URL 内容注入上下文

### 13. Session 管理
- 对话存档（`/save` → JSON）、恢复（`/load`）
- 支持分支对话（相同起点不同方向）

### 14. Plan 模式
- 复杂改动先出方案文本，用户审批通过后再执行
- plan 可编辑、可引用

### 15. 凭证管理
- API key 安全存储（keyring / 加密文件）
- 多 profile 切换（work / personal）

### 16. 自动化测试
- Agent 行为回归测试（mock LLM 响应，验证工具调用正确性）
- 工具执行单元测试（mock 文件系统和 shell）
