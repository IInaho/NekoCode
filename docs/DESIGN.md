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
| `/clear` | 清空对话历史和摘要 |
| `/stats` | 查看上下文状态：消息数、tokens、是否有摘要 |
| `/summarize` | 手动触发上下文压缩，返回压缩前后对比 |
| `/config` | 显示当前 provider 和 model |

## TUI 界面设计

### 视觉主题：深夜书房

黑猫蜷在屏幕旁的意象——teal 色偶尔闪现，像暗处的猫眼。

**色彩体系**：
- 背景：终端原生黑色
- 主文字：`#a0a0a0` 中性灰
- 主色 teal：`#4ec9b0`，用于 Assistant 色条、spinner
- User 金：`#c9a96e` 暗金
- 弱化文字：`#666666`
- 边框线：`#333333`
- 工具卡：暖金 `#c9a96e`

### 启动页

```
          /\___/\
         ( ◉   ◉ )
          =  ▾  =
         /|     |\
        (_|     |_)
           || ||

           PRIMUS
          v0.1.0

      ──── ◆ ────

         Press Enter
```

猫眼 `◉` 闪烁 teal 光，像黑猫在暗处睁眼。用户按下 Enter 进入聊天界面。

### 聊天界面布局（Plan B：厚左色条）

```
(=^.^=) PRIMUS v0.1.0 · 1.2k/128k · anthropic/claude

▐ You                                                        ┃
▐ 帮我分析下项目架构                                           ┃

▐ Assistant                                                  ┃
▐                                                            ┃
  ┌ ◆ filesystem list .  [+] ──────────────────────────┐     ┃
  │ ◆ filesystem read main.go [+]                       │     ┃
  └──────────────────────────────────────────────────────┘     ┃
▐                                                            ┃
▐ ## 项目架构                                                 ┃
▐                                                            ┃
▐ 这是一个 Go 编写的终端 AI 助手...                             ┃
▐                                                            ┃
▐ Duration: 12.3s  ↑670 ↓128                                 ┃
```

- **左侧**：`▐`（U+2590）厚色条 + `PaddingLeft(1)` 统一缩进
- **右侧**：独立 Scrollbar 组件，`┃` thumb + `│` track
- **工具卡片**：暖金色 `NormalBorder`，聚合所有工具在一个卡片，`[+]`/`[-]` 折叠
- **思考文本**：`💭` 前缀 + Subtle 色，多行自动对齐
- **Footer**：Subtle 色 Duration + Token 统计，不经 glamour

### 处理阶段

```
▐ ◉ Reasoning (3.2s) ↑670 ↓56               ← LLM 生成文本中

▐   💭 好呀～让我来帮你分析一下项目架构喵！   ← 思考文本
▐   ┌ ◆ filesystem list .  [+] ──────────┐  ← 工具卡片
▐   └────────────────────────────────────┘
▐   💭 好多文件～继续深入看看                ← 新一轮思考

▐ ◉ Running bash (0.3s)                    ← 工具执行中

▐ Assistant                                ← 完成
▐   完整回答（glamour 渲染）                 ← 最终回复
```

阶段流转：Ready → Thinking → Reasoning → Running → Thinking → ... → Ready

- **Thinking**: LLM 调用已发出，等待响应
- **Reasoning**: 首个 token 到达，模型生成中
- **Running**: 工具执行中

### 工具确认栏

```
Confirm
  bash rm -rf /tmp/cache  [destructive]
  Proceed?  [enter] yes  [esc] no
```

- `[write]` 黄色，`[destructive]` 红色
- Enter 确认，Esc 取消
- 禁止级操作不弹框，直接告知被拒绝

### 输入交互

- **发送**：Enter 提交，输入框清空，消息即时显示
- **历史翻阅**：↑/↓ 翻阅历史
- **命令提示**：输入 `/` 弹出命令列表，Tab/Shift+Tab 选择

## Agent 能力

### 工具清单

| 工具 | 功能 | 安全等级 | 执行模式 |
|------|------|----------|----------|
| **bash** | Shell 命令 | 四级匹配 | Sequential |
| **filesystem** | 文件读写列表 | read/list Safe, write 确认 | Parallel(read)/Seq(write) |
| **glob** | 文件模式匹配 | Safe | Parallel |
| **grep** | ripgrep 搜索 | Safe | Parallel |
| **edit** | 精确字符串替换 | Write | Sequential |
| **web_search** | Exa AI 搜索 (Bing 降级) | Safe | Parallel |
| **web_fetch** | 网页抓取 | Safe | Parallel |

### 危险命令分级

**自动放行**：`ls`, `cat`, `pwd`, `grep`, `find`, `head`, `tail`, `wc` 等

**确认后执行（写）**：`mkdir`, `touch`, `cp`, `mv`, `git add/commit`, `npm install` 等

**确认后执行（危险）**：`rm`, `chmod`, `kill`, `reboot`, `git push --force` 等

**直接拒绝**：`sudo`, `eval`, `ssh`, `telnet`, `dd`, `mkfs` 等

### 并行工具执行

互不依赖的工具（glob + grep + web_search）由 executor 并发执行，worker pool 上限 10。任一工具声明 `ModeSequential` 则整批串行。

### 思考过程展示

工具调用间的 LLM 文本以 `💭` 思考块展示——不经过 glamour，纯 Subtle 色 prose 文本。只展示中间过程，最终回答用 glamour 渲染。

## 长对话管理

- **滑动窗口**：保留最近 20 条消息
- **结构化自动摘要**：消息 > 20 且 token > budget/2 触发，压缩为六段结构化格式（目标/进展/关键决策/下一步/关键上下文/相关文件），支持增量更新
- **Token 保护**：Build 时从最早消息丢弃，保护 tool_calls/tool_result 配对
- **手动清空**：`/clear` 清空消息和摘要
- **BTW 中断**：Processing 中可直接打字 + Enter 打断当前 LLM 调用并注入新消息，Esc 纯 abort

## 角色设定

软萌二次元黑猫少女，语气可爱（呀、呢、啦、喵），风格元气治愈。

工具使用规则（system prompt 末尾）：
- grep 优先、并行搜索、先读后搜、及时收手
- 图表用代码块包裹
- 优先用 markdown 标题+列表，需要空间关系才用 ASCII 图

## 模块职责

| 模块 | 位置 | 职责 |
|------|------|------|
| **Agent 循环** | `bot/agent/` | Reason→Execute→Feedback，并行调度，BTW 中断，指数退避重试 |
| **LLM 网关** | `llm/` | 统一对接多 provider，共享 HTTP 连接池，流式解析 |
| **工具系统** | `bot/tools/` | Tool 接口 + ExecutionMode + DangerLevel + ANSI 清理 |
| **上下文管理** | `bot/ctxmgr/` | 4 文件拆分，Hybrid Window+Structured Summary，语言感知 token 估算 |
| **确认机制** | `bot/types/` | channel 通信，TUI 渲染确认栏 |
| **Bot 组装** | `bot/bot.go` | 依赖注入，ShouldStop，ContextTransform |
| **命令系统** | `bot/command.go` | `/` 前缀解析，handler 注册 |
| **配置** | `bot/config.go` | `~/.primusbot/config.json` |
| **TUI** | `tui/` | Bubble Tea v2，BotInterface 解耦，21 文件组件化，Plan B 视觉 |

## 非交互模式

```bash
primusbot "帮我看看当前目录有什么文件"
```
