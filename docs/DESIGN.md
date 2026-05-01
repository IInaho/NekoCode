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

一只黑猫蜷在屏幕旁的意象贯穿始终。猫不是画出来的，是通过颜色暗示的——teal 色偶尔闪现，像暗处的猫眼。

**色彩体系**：
- 背景：终端原生黑色
- 主文字：`#a0a0a0` 中性灰
- 主色 teal：`#4ec9b0`，仅用于 spinner、角色标签等点缀
- User 金：`#c9a96e` 暗金，不刺眼
- Assistant 绿：`#4ec9b0` teal
- 弱化文字：`#666666`
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
          v0.1.0

      ──── ◆ ────

         Press Enter
```

启动瞬间猫眼 `◉` 会闪烁 teal 光（暗→亮→暗→亮定格），像黑猫在暗处睁眼。用户按下 Enter 进入聊天界面后，猫消失——只留下 teal 色在 spinner 和角色标签中间歇出现，暗示"猫还在看着"。

### 聊天界面布局

```
(=^.^=) PRIMUS v0.1.0 · 1.2k/64k · openai/gpt-4
──────────────────────────────────────────

│ You
│ 帮我看看 main.go 的内容

│ Assistant
│ > 调用工具: bash `bash(command=cat main.go)`
│
│ 主人～这是 main.go 的内容喵！(´▽`ʃ♡ƪ)
│
│ package main
│ ...
│ 看起来是个很厉害的 Go 程序呢～

──────────────────────────────────────────
┃ Type a message...
──────────────────────────────────────────
```

**顶部 Header**（1 行）：猫脸图标 + 应用名 + 版本 + token 用量 + provider/model，底部细线分隔。token 用量实时更新，颜色按消耗比例变化：灰色 (<60%)、黄色 (60-90%)、红色 (≥90%)。

**消息区域**（主体）：
- 每条消息左侧一条竖线 + 角色标签
- User 金色竖线，Assistant teal 竖线
- 无顶线底线的完整边框，视觉轻盈
- 消息间无额外空行，竖线本身提供分隔
- Assistant 的思考/工具过程以更暗色（`#3a6a5a`）渲染在正文上方，可折叠感知

**底部 Input**（2 行）：一条分隔线 + 提示符 `┃` + 输入区域 + 一条底部边界线。

### 工具确认栏

当助手准备执行写操作或危险操作时，消息区域底部出现确认栏：

```
[safe] bash command=ls                                         [enter] confirm  [esc] cancel
[write] filesystem path=test content=hello cat                 [enter] confirm  [esc] cancel
[destructive] bash command=rm -rf /tmp/cache                   [enter] confirm  [esc] cancel
```

- 安全等级标签着色：`[write]` 黄色，`[destructive]` 红色
- 显示工具名 + 参数概要
- Enter 确认执行，Esc 取消
- 确认期间界面静止（spinner 暂停），等待用户决定
- 禁止级操作（`[forbidden]`）不弹框，直接在聊天中告知被拒绝

### 流式输出

助手回复以结构化内容块实时展示：
- spinner `◉` + 阶段状态（"Thinking (2.1s)" / "Running bash (3.5s)"）
- 工具调用以独立卡片呈现，`◆` 暖金色图标，可折叠 `[+]`/`[-]`
- 完成后卡片保留在消息中，`ctrl+e` 切换展开/折叠

### 输入交互

- **发送**：Enter 提交消息，输入框清空
- **历史翻阅**：空输入框时按 ↑/↓ 翻阅发送历史
- **命令提示**：输入 `/` 自动弹出可用命令列表，支持实时过滤。Tab / Shift+Tab 选择，Enter 填入（光标移到末尾可继续输入参数），再按 Enter 发送。精确匹配时不弹框

**提示框外观**：
```
── suggestions ──
> /help            ← teal 高亮选中项
  /clear
  /summarize
  /stats
  /config
```
- **退出**：Ctrl+C

## Agent 能力

### 工具清单

| 工具 | 功能 | 安全等级 |
|------|------|----------|
| **bash** | 执行 Shell 命令 | 三级关键词匹配：safe/write/destructive/forbidden |
| **filesystem** | 文件读写列表 | `read`/`list` 自动放行，`write` 需确认 |
| **glob** | 文件模式匹配 | 始终自动放行 |
| **grep** | ripgrep 内容搜索 | 始终自动放行 |
| **edit** | 精确字符串替换 | 需确认，失败返回文件内容 |

### 危险命令分级

**自动放行**（只读）：
`ls`, `cat`, `pwd`, `grep`, `find`, `head`, `tail`, `wc`, `file`, `stat` 等

**确认后执行**（写操作）：
`mkdir`, `touch`, `cp`, `mv`, `tar`, `zip`, `git add`, `git commit`, `pip/npm/go install`, `make` 等

**确认后执行**（危险）：
`rm`, `chmod`, `chown`, `kill`, `pkill`, `shutdown`, `reboot`, `git push`, `git reset --hard` 等

**直接拒绝**（禁止）：
`sudo`, `eval`, `curl|bash`, `ssh`, `telnet`, `nc`, `dd`, `mkfs` 等

### 决策流程

1. 用户输入到达 → 助手判断是 `/` 命令还是自然语言
2. 自然语言进入推理：LLM 根据对话历史和可用工具决定是直接聊天还是调用工具
3. 若调用工具：检查危险等级 → 必要时弹出确认栏 → 执行 → 将结果回传 LLM 生成最终回复
4. 若直接聊天：LLM 直接生成回复
5. 回复展示给用户，本轮结束

### 任务连续性

助手能执行多步操作。例如"帮我看看有哪些 Go 文件，然后统计下代码行数"：
1. 调用 glob 找到所有 `.go` 文件
2. 调用 bash `wc -l *.go` 统计行数
3. 生成总结回复

## 长对话管理

助手会自动管理对话长度，确保不会超出模型 token 上限：

- **滑动窗口**：始终保留最近 20 条消息原文，超出的旧消息进入待压缩区
- **自动摘要**：当消息数量超过窗口且 token 超过预算一半时，触发摘要——将最早的消息传给 LLM，压缩为 ≤300 字的摘要
- **摘要合并**：多次摘要会累积更新，保持信息连续性
- **Token 保护**：即使未触发摘要，Build 时也会从最早消息开始丢弃，保证不超预算
- **清空重来**：`/clear` 同时清空消息和摘要

用户不会感知到这个过程——对话体验始终连贯。

## 角色设定

助手以软萌二次元美少女风格回复（通过 system prompt 控制）。说话带可爱语气词（呀、呢、啦、喵），风格元气治愈，行为乖巧懂事。

工具调用阶段，系统在角色 prompt 末尾附加一条简短的"有操作需求时请调用函数"指令。Native Function Calling 将工具定义作为独立参数传入，与文本上下文分离，既保证操作准确性，又不影响对话风格。

## 模块职责一览

| 模块 | 位置 | 职责摘要 |
|------|------|----------|
| **Agent 循环** | `bot/agent/` | Reason → Execute → Feedback 三阶段循环，状态由 stepState 在迭代间传递 |
| **LLM 网关** | `llm/` | 统一对接 OpenAI / Anthropic / GLM，原生 Function Calling，屏蔽 provider 差异 |
| **工具系统** | `bot/tools/` | Tool 接口 + Registry + DangerLevel 四级安全分级 + ParseCall 调用协议 |
| **上下文管理** | `ctxmgr/` | Hybrid Window + Summary：Build(withTools) 统一入口，自动摘要，token 截断 |
| **确认机制** | `bot/types/types.go` | ConfirmRequest channel 通信，TUI 渲染确认栏，enter/esc 应答 |
| **Bot 组装** | `bot/bot.go` | 依赖注入：ctxmgr + agent + tools + llm + command，SummarizeIfNeeded 触发 |
| **命令系统** | `bot/command.go` | `/` 前缀解析，handler 注册模式，CommandCallbacks 依赖注入 |
| **配置管理** | `bot/config.go` | `~/.primusbot/config.json` 加载 |
| **TUI 界面** | `tui/` | Bubble Tea v2，通过 `BotInterface` 接口与 bot 解耦；components 子包渲染消息/输入/启动页；styles 子包管理色彩和 Markdown |

## 非交互模式

除了 TUI，也支持命令行直接调用：

```bash
primusbot "帮我看看当前目录有什么文件"
```

同样走 Agent 循环，输出结果直接打印到 stdout。
