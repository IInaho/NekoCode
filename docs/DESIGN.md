# PrimusBot 设计文档

## 产品定位

PrimusBot 是一个运行在终端中的 AI 助手。它能理解自然语言、执行本地操作（文件读写、Shell 命令、文件搜索），并在执行可能有影响的操作前征求用户确认。

核心体验：**像和一位终端里的伙伴聊天一样，自然地交代任务，它帮你完成。**

## 交互模式

### 闲聊模式

用户说"你好"、"最近怎么样"等纯对话内容时，助手以自然语言回复，不触发任何工具。

### 任务模式

用户说"帮我看看 main.go 的内容"、"列出当前目录"等操作请求时，助手自动选择合适的工具执行，并将结果以自然语言呈现。

**用户无需区分模式**——助手的内部决策自动判断该聊天还是该操作。

### 斜杠命令

以 `/` 开头的输入为系统命令：

| 命令 | 效果 |
|------|------|
| `/help` | 显示可用命令列表 |
| `/new` | 开始新对话（保留上一任务摘要） |
| `/clear` | 清空所有对话历史和摘要 |
| `/stats` | 查看上下文状态：消息数、tokens、是否有摘要 |
| `/summarize` | 手动触发上下文压缩，返回压缩前后对比 |
| `/config` | 显示当前 provider 和 model |

## TUI 界面设计

### 视觉主题：深夜书房

黑猫蜷在屏幕旁的意象——teal 色偶尔闪现，像暗处的猫眼。

**色彩体系**（`tui/styles/colors.go` 统一定义）：
- 主文字：`#a0a0a0`
- Teal 主色：`#4ec9b0`（styles.Primary），用于 Assistant 色条、spinner
- User 金：`#c9a96e`（styles.Yellow）
- 蓝：`#7a8ba0`（styles.Blue）
- 红：`#e06c75`（styles.Red）
- Diff 绿：`#98c379`（styles.DiffGreen）
- Diff 灰：`#5c6370`（styles.DiffSubtle）
- 弱化文字：`#666666`，中间：`#808080`
- 边框线：`#333333`

### 启动页

```
          /\___/\
         ( ◉   ◉ )
          =  ▾  =
         /|     |\
        (_|     |_)
           || ||

           PRIMUS
          v0.2.0

      ──── ◆ ────

         Press Enter
```

猫眼 `◉` 闪烁 teal 光。用户按下 Enter 进入聊天界面。

### 聊天界面布局（厚左色条）

```
(=^.^=) PRIMUS v0.2.0 · anthropic/claude

▐ You                                                        ┃
▐ 帮我分析下项目架构                                           ┃

▐ Assistant                                                  ┃
▐                                                            ┃
  ┌ ◆ read ×5 [+] 展开 ─────────────────────────────────┐     ┃
  │ ◆ grep "func" .  [+]                                │     ┃
  └──────────────────────────────────────────────────────┘     ┃
▐                                                            ┃
▐ ## 项目架构                                                 ┃
▐ ...                                                        ┃
▐ Duration: 12.3s  ↑670 ↓128                                 ┃
```

- **左侧**：`▐`（U+2590）厚色条 + `PaddingLeft(1)` 统一缩进
- **右侧**：独立 Scrollbar 组件，`┃` thumb + `│` track
- **工具卡片**：暖金色 `NormalBorder`，edit 块显示 `[+]`/`[-]` 折叠展开 diff
- **工具组**：同名单行工具折叠为 `◆ read ×5 [+]`，展开后逐条显示
- **处理卡片**：teal 边框，分隔线横跨全宽区分 output/reasoning 区块

### 处理阶段

```
▐ ◉ Thinking (3.2s) ↑670 ↓56 🧹3    ← 当前阶段 + 耗时 + token + 微压缩计数

▐   ▍ output ──────────────────────   ← 分隔线（teal）
▐   正在分析项目结构...                ← 模型流式输出（动态 2-6 行）

▐   ▍ reasoning ───────────────────   ← 分隔线（蓝色）
▐   让我读取所有源文件来分析...        ← 推理过程（动态 2-6 行）

▐   ◆ glob ×2 [-] 收起                ← 收折工具组
▐     ◆ glob *.go                     ← 展开：逐条显示
▐     ◆ glob *.md
```

阶段流转：Waiting → Thinking → Reasoning → Running → Thinking → ... → Ready

- **Waiting**: LLM 调用已发出，等待首 token
- **Thinking**: ReasoningContent 到达（DeepSeek CoT）
- **Reasoning**: Content token 到达，模型生成文本中
- **Running**: 工具执行中
- **🧹N**: 累计微压缩清除的工具结果数

### 工具确认栏

```
Confirm
  bash rm -rf /tmp/cache  [destructive]
  Proceed?  [enter] yes  [esc] no
```

- `[write]`/`[destructive]` 黄色，`[forbidden]` 红色（直接拒绝不弹框）
- Enter 确认，Esc 取消

### 输入交互

- **发送**：Enter 提交，消息即时显示
- **处理中输入（BTW）**：Enter 注入新消息打断当前 LLM 调用
- **历史翻阅**：↑/↓ 翻阅历史
- **命令提示**：输入 `/` 弹出命令列表，Tab/Shift+Tab 选择
- **块切换**：Ctrl+E 展开/收起工具组和 edit diff

## 上下文管理

### 三层策略

| 层 | 触发条件 | 动作 |
|----|---------|------|
| **微压缩** | token > 50% 预算 | 清除旧 compactable 工具结果（read、grep、glob 等），保留最近 5 个 |
| **结构化摘要** | token > 80% 预算 + 消息超窗口 | LLM 生成结构化摘要压缩最旧消息 |
| **Snip 剪枝** | 模型主动触发 | 模型调用 snip 工具移除旧消息范围 |

### Session Memory

上下文超过 10k token 后开始异步提取，每 +5k token + 3 个 tool call 再次触发。提取内容写入 `~/.primusbot/sessions/<id>/memory.md`（10 section Markdown 文件）。`/new` 命令优先用 session memory 作为免费摘要。

## Agent 能力

### 工具清单

| 工具 | 功能 | 安全等级 | 执行模式 |
|------|------|----------|----------|
| **bash** | Shell 命令 | 全部 Write+（确认） | Sequential |
| **read** | 文件读取（文本/图片/PDF） | Safe | Parallel |
| **write** | 文件创建/覆盖（路径校验） | Write | Sequential |
| **edit** | 精确字符串替换 + diff | Write | Sequential |
| **list** | 目录列表 | Safe | Parallel |
| **glob** | 文件模式匹配（支持 **） | Safe | Parallel |
| **grep** | ripgrep 内容搜索 | Safe | Parallel |
| **web_search** | Exa MCP 搜索 | Safe | Parallel |
| **web_fetch** | 网页抓取 | Safe | Parallel |
| **snip** | 移除旧消息 | Destructive | Sequential |
| **task** | 子 agent 委派 | Safe | Parallel |
| **todo_write** | 任务列表更新 | Safe | Sequential |

### 子 Agent 类型

| 类型 | 用途 | 工具 | 最大步数 |
|------|------|------|----------|
| executor | 执行编码任务 | read/write/edit/bash/grep/glob/list | 4 |
| verify | 对抗性验证 | read/grep/glob/list/bash | 6 |
| explore | 代码探索/调研 | read/grep/glob/list/web_search/web_fetch | 2 |
| plan | 方案设计 | read/grep/glob/list/web_search/web_fetch | 3 |
| decompose | 任务拆解 | read/grep/glob/list | 2 |

子 agent 通过独立 LLM 客户端运行，disableThinking=true，上下文隔离，共享 `tools.Executor` 保证安全检查一致。

### 危险命令分级

所有 bash 命令默认 `LevelWrite`（需确认），然后按关键词匹配升级：

**升级至 Destructive（危险）**：`rm`、`chmod`、`kill`、`reboot`、`git push --force` 等
**升级至 Forbidden（拒绝）**：`sudo`、`eval`、`ssh`、`curl|bash`、`dd`、`mkfs` 等

### 并行工具执行

互不依赖的工具并发执行，worker pool 上限 10。并行启动前检查 ctx 取消状态。subagent 共享同一个 Executor 实例。

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

## 非交互模式

```bash
primusbot "帮我看看当前目录有什么文件"
```
