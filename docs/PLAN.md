# PrimusBot 开发路线

按优先级排列，每项可独立实施验证。✅ = 已完成。

---

## P0 — 核心功能

### 1. 精确编辑工具 (EditTool) ✅
- 精确替换文件中首次出现的字符串
- 失败时返回带行号的文件内容

### 2. 内容搜索工具 (GrepTool) ✅
- 基于 ripgrep 的内容搜索，支持 regex/glob/context

### 3. Diff 展示 ✅
- EditTool 执行后返回 +- 行级对比
- TUI markdown 渲染 diff（+ 绿色，- 红色）

### 4. 结构化内容块 (ContentBlock) ✅
- 替代纯文本流，工具调用/思考/文本用独立 block 渲染
- 每个 tool block 可独立折叠 `[+]`/`[-]`
- `ctrl+e` 翻转最后一条消息的所有 tool block

### 5. TUI-Bot 解耦 ✅
- `BotInterface` 接口：TUI 仅依赖 8 个方法
- `bot/types` 共享类型包：ConfirmRequest / ConfirmFunc / PhaseFunc
- Bot 内部字段私有化，通过 getter 暴露

### 6. 项目感知上下文
- 启动时自动加载项目信息注入 system prompt：
  - 读取 `CLAUDE.md` / `AGENTS.md`（项目规则、编码规范）
  - 读取 `.gitignore`（排除文件）
  - 生成目录树摘要

### 7. Skills 系统
- 可动态注册的能力模块
- 每个 skill 定义：名称、描述、触发条件、关联工具集

---

## P1 — 架构增强

### 8. Provider 合并 ✅
- OpenAI / GLM / DeepSeek 合并为 `OpenAICompatible`
- 删除 ~130 行重复 HTTP/流式代码

### 9. 上下文窗口优化 ✅
- tokenBudget 默认 64000，可在 config.json 配置
- Build() 截断时保护 tool_calls/tool_result 配对

### 10. 确认框重构 ✅
- Claude 风格卡片式布局：Tool / File / Level + 选项行
- 高度固定 4 行，通过 resizeMessages() 动态调整视口

### 11. 子 Agent 并行
- 复杂任务拆分为独立子任务，并行执行，结果汇总

### 12. 后台任务 + 进度
- 长运行命令流式输出，不阻塞主 Agent 循环

### 13. Checkpoint / Undo
- 每次工具写入前自动保存快照
- `/undo` 命令回滚

### 14. 任务列表 (Todo tracking)
- 复杂请求自动生成 Todo，TUI 实时显示进度

---

## P2 — 生态与体验

### 15. MCP 协议支持
- MCP client，连接外部 tool server

### 16. Web Search / Fetch ✅
- `web_search`：Bing HTML 抓取，零配置零 API key，国内可用
- `web_fetch`：HTTP GET + HTML→Markdown 转换，支持 prompt 指导提取
- [技术方案](./design/web-search.md)

### 17. Session 管理
- 对话存档/恢复，支持分支对话

### 18. Plan 模式
- 复杂改动先出方案文本，用户审批后执行
- [技术方案](./design/plan-mode.md)

### 19. 凭证管理
- API key 安全存储，多 profile 切换

### 20. 自动化测试
- Agent 行为回归测试（mock LLM 响应）
- 工具执行单元测试（mock 文件系统/shell）
