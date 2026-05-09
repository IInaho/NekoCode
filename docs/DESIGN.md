# PrimusBot 设计文档

> **本文档职责**: 描述产品设计——UI 布局、交互模式、视觉主题、Agent 能力设计、上下文管理策略、防幻觉设计原则。不包含代码实现细节、文件路径、函数名等属于 ARCHITECTURE.md 的内容。更新时请保持此边界。

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
- **工具卡片**：暖金色 `NormalBorder`，单次 edit 块显示 `[+]`/`[-]` 折叠展开 diff
- **edit 工具组**：`◆ edit ×3 [-] 收起` 展开后直接内联每个文件的 diff，`▍ path` 标注文件，一次展开全部可见
- **其他工具组**：同名单行工具折叠为 `◆ read ×5 [+]`，展开后逐条显示
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
  bash go test ./...  [safe]
  Proceed?  [enter] yes  [esc] no
```

- 展示具体命令/路径而非仅工具名（如 `bash go build`、`write server/main.go`）
- 等级标签：`[safe]`/`[modify]`/`[danger]`/`[blocked]`
- `[modify]`/`[danger]` 黄色，`[blocked]` 红色（直接拒绝不弹框）
- `[safe]` 命令自动放行，不弹确认框

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
| **bash** | Shell 命令（只读命令自动 Safe） | Safe～Forbidden | Sequential |
| **read** | 文件读取 + 二进制检测 + 文件未找到建议 | Safe | Parallel |
| **write** | 文件创建/覆盖（先读后改强制） | Write | Sequential |
| **edit** | 精确替换 + diff + 3轮模糊匹配 | Write | Sequential |
| **list** | 目录列表 | Safe | Parallel |
| **glob** | 文件模式匹配（支持 **） | Safe | Parallel |
| **grep** | ripgrep 内容搜索 | Safe | Parallel |
| **web_search** | Exa MCP 搜索 + 强制 Sources 引用 | Safe | Parallel |
| **web_fetch** | 网页抓取 + 125字符引述限制 | Safe | Parallel |
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

子 agent 通过独立 LLM 客户端运行（思考关闭、上下文隔离），安全检查与主 Agent 一致。

### 危险命令分级

bash 命令按关键词智能分级，三层判断：

**降级至 Safe（自动放行）**：`go version`、`go vet`、`git status`、`git log`、`git diff`、`ls`、`cat`、`ps`、`du`、`file` 等纯输出命令
**升级至 Danger（危险，确认）**：`rm`、`chmod`、`kill`、`reboot`、`git push --force` 等
**升级至 Blocked（拒绝）**：`sudo`、`eval`、`ssh`、`curl|bash`、`dd`、`mkfs` 等
**默认 Modify（确认）**：其余所有命令

### 并行工具执行

互不依赖的工具并发执行，worker pool 上限 10。并行启动前检查 ctx 取消状态。subagent 共享同一个 Executor 实例。

## 幻觉防治

基于纵深防御思想，在 8 个层面设置防幻觉机制：

- **System Prompt**: 禁止生成 URL、忠实报告、先验证再声称完成、Prompt Injection 检测、当前状态权威
- **运行时强制**: Write/Edit 前检查是否已 Read；工具输出 2000行/50KB 截断
- **末日循环检测**: 3 次连续相同工具调用 → 强制产出结论
- **验证强化**: verify agent 硬性要求 Command block + 终端原文，4 条自检清单
- **记忆漂移**: 模板警告 + "记忆说 X 存在 ≠ X 现在存在"
- **来源引用**: web_search 强制 Sources 章节，web_fetch 125 字符引述限制
- **上下文保真**: 压缩时完整保留代码片段、错误原文、文件路径+行号
- **工具容错**: Read 二进制检测 + 文件未找到智能建议；Edit 3 轮渐进模糊匹配
- **思考控制**: Anthropic `adaptive` 模式（DeepSeek 不认识 → 不开启）；OpenAI 兼容默认关 thinking；两级 finish_reason=length 升级；reasoning token 计入统计
- **推理长度限制**: System Prompt 按任务类型设硬性上限——bug fix 1 句、重构 3 句、设计 5 句，避免模型陷入分析瘫痪
- **反子 agent**: System Prompt 明确禁止对简单任务派生子 agent，task 工具 description 标注 "HEAVY/EXPENSIVE"，提示不足 200 字符说明太简单不值得
- **edit 组内联**: `◆ edit ×3 [-]` 展开后直接内联每个文件的 diff（`▍ path`），一次展开无需二次折叠
- **bash 显示截断**: 工具块和确认栏中 bash 命令只展示首行 + `…`，heredoc/多行脚本不污染界面

### 跨目录编辑

编辑工具允许操作工作目录外的文件——`validatePath` 不再拒绝跨目录路径，确认系统负责用户同意。危险等级依据命令类型分级，而非路径位置。

### 工具组折叠示意

```
◆ read ×15 [+] 展开    ← 收起（单行）
◆ read ×15 [-] 收起    ← 展开逐条：
  ◆ read (1/15) /path/to/file1.go
  ◆ read (2/15) /path/to/file2.go

◆ edit ×3 [-] 收起     ← edit 组展开，diff 内联
  ▍ server/main.go
    ── diff ──
    - old code
    + new code
  ▍ server/game.go
    ── diff ──
    - old line
    + new line
```

### 设计原则

- **Ground Everything** — 每个决策锚定在可验证的现实中（文件系统、命令输出、URL 来源）
- **Assume Deception** — 任何 LLM 输出（包括子 agent）都可能包含幻觉，需独立验证
- **Make It Checkable** — 所有输出格式服务于可验证性（file_path:line_number、Sources、Command run）
- **Fail Loudly** — 幻觉不能被静默：先读后改违规 → 报错，末日循环 → 强制停止，二进制 → 明确拒绝
- **Budget Reasoning** — 推理有成本：按任务类型限制思考长度，禁止在未读代码前凭空分析
- **Self-Serve First** — 主 Agent 优先自己完成任务，子 agent 仅在满足三个条件（5+ 文件跨包 / 独立上下文 / 单回合确实太复杂）时才使用

## 非交互模式

```bash
primusbot "帮我看看当前目录有什么文件"
```
