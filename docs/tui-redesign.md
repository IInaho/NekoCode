# PrimusBot TUI 重构设计文档

## 一、设计主题

**视觉基调**: 深夜书房 — 一只黑猫蜷在屏幕旁，眼睛偶尔闪一下 teal 光。

猫不是画出来的，是**暗示**出来的：颜色、呼吸感、微妙的光点。终端里画猫 ASCII art 只在启动页出现一次，其余时间猫以"隐形"的方式存在。

**核心记忆点**: 启动时那只猫的眼睛会闪一下（teal 光标闪烁），之后猫消失，只留下颜色暗示。

---

## 二、色彩体系重构

### 问题

当前配色偏 VS Code 默认主题（#4ec9b0 / #569cd6 / #dcdcaa），teal 作为主色过于抢眼，缺乏"深夜"氛围。

### 方案

从"teal 主导"改为"深色主导 + teal 点缀"：

```
层级          当前            重构后
─────────────────────────────────────────────
背景基调      无（终端黑）     无（终端黑）
文字主体      #808080         #a0a0a0  ← 提亮，增强可读性
文字弱化      #5a5a5a         #666666
边框线        #404040         #333333  ← 更深，退到背景里
主色(teal)    #4ec9b0         #4ec9b0  ← 保留，但减少使用面积
猫眼色        #4ec9b0         #7ec8e3  ← 偏冷蓝，区分于主色
User 边框     #dcdcaa         #c9a96e  ← 暗金，低调
Assistant     #4ec9b0         #4ec9b0  ← 保持
System        #569cd6         #7a8ba0  ← 灰蓝，不抢眼
Error         #f44747         #e06c75  ← 柔和红
```

**关键变化**: 主色 teal 的使用面积从"边框 + 标题 + spinner + 提示符"缩减到仅"角色标签 + spinner"。边框线全部降到 #333333，消息框退为背景的一部分。

### 实现

`ui/styles/styles.go` 修改颜色常量，新增 `fgText = "#a0a0a0"` 替代 `fgMuted` 作为文字主体色。

---

## 三、消息气泡重构

### 问题

当前每条消息都有完整的 box-drawing 边框（╭─ / │ / ╰──），4 种颜色 × 3 行 = 视觉噪音大。消息密集时像表格而非对话。

### 方案

**去掉完整边框，改为左侧色条 + 缩进**：

```
当前:
╭─ You ─────────────────────
│ 你好，帮我看看这段代码
╰───────────────────────────

重构后:
┃ You
┃ 你好，帮我看看这段代码

  （空行分隔）
```

具体做法：

1. **左侧只保留一条竖线 `│`**，用角色颜色渲染
2. **角色标签** 在第一行竖线后，加粗
3. **去掉顶线和底线**（╭─ / ╰──），减少视觉重量
4. **内容行** 缩进 2 格（竖线 + 空格）
5. **消息间距** 从 1 行空行改为 0（竖线本身已提供视觉分隔）

### 视觉对比

```
当前（每条消息 3 行边框）:             重构后（纯左侧色条）:

╭─ You ──────────────                 │ You
│ 你好                                │ 你好
╰────────────────────
                                       │ Assistant
╭─ Assistant ────────                 │ 这是回复内容，
│ 这是回复内容，很长很长               │ 很长很长会自动
│ 会自动换行                           │ 换行
╰────────────────────
```

信息密度提升约 40%，视觉噪音大幅降低。

### 特殊消息

- **Assistant reasoning**: 竖线颜色从 teal 变为更暗的 `#3a6a5a`（暗 teal），内容用 subtle 色，与正文区分
- **Error**: 竖线保持红色，但标签从 `Error` 改为 `!`，更紧凑
- **System**: 竖线保持灰蓝，标签从 `System` 改为 `·`

---

## 四、Header 重构

### 问题

当前 Header 是一个完整的 box 框（5 行），占用大量垂直空间，且与 Footer 形成"夹击"感。

### 方案

**压缩为 1 行状态栏**：

```
当前（5 行）:                          重构后（1 行）:

╭────────────────────╮               (=^.^=) PRIMUS v0.1.0  ·  openai/gpt-4
│ (=^.^=) PRIMUS     │
├────────────────────┤
│ Provider: oai/gpt4 │
├────────────────────┤
```

做法：
1. 猫脸图标 `(=^.^=)` 用 CatBody + CatEye 样式
2. `PRIMUS` 用 PrimaryStyle Bold
3. 版本号用 SubtleStyle
4. Provider/Model 用 `·` 分隔，MutedStyle
5. 右侧显示 Follow 状态（从 Footer 迁移）
6. 底部一条细线 `─` 分隔，用 BorderStyle

**Footer 移除**：快捷键提示改为在 Input 区域的 Placeholder 中轮播显示，或仅在 `/help` 时展示。

### 收益

- 节省 4 行垂直空间给消息区域
- 视觉更干净，"框架感"消失
- 猫的存在感从"画了一只猫"变为"猫脸图标在角落看着你"

---

## 五、Input 区域重构

### 问题

当前 Input 是 textarea 原生样式，与整体风格割裂。

### 方案

**将 Input 嵌入底部框架**：

```
当前:                                  重构后:

┃ Type a message...                    ──────────────────────
├────────────────────                  ┃ Type a message...
│ Scroll · Follow:Auto                 ──────────────────────
╰────────────────────
```

做法：
1. Input 上方加一条细线 `─`（BorderStyle），与消息区域分隔
2. 提示符 `┃` 改为猫爪 `🐾`（Unicode 降级为 `>`）
3. Placeholder 内容轮播快捷键提示：
   - 默认: `Type a message...`
   - 3 秒无输入: `Enter to send · ↑↓ to scroll`
   - 再 3 秒: `Ctrl+C to quit · /help for commands`
4. Input 下方加一条细线，作为整个 UI 的底部边界

### 猫爪提示符

```go
// prompt 字符
PromptPaw = "🐾"  // Unicode
PromptPaw = ">"   // ASCII fallback
```

如果猫爪太可爱不合适，备选方案是用 `▸` 或保持 `┃` 但颜色改为 CatEye 冷蓝。

---

## 六、Processing 指示器重构

### 问题

当前 `◉ ⠋ Thinking...` 视觉上像一个普通文本行，没有"正在处理"的紧迫感。

### 方案

**左侧色条 + 脉冲效果**：

```
当前:                                  重构后:

◉ ⠋ Thinking...                       │ ◉ Thinking...
                                       │   (流式文本逐步出现...)
  (流式文本...)
```

做法：
1. ProcessingItem 也使用左侧竖线，颜色为 teal（与 Assistant 一致）
2. 标签从 `◉ Thinking...` 改为 `◉ Thinking`，去掉省略号（spinner 本身已表达进行中）
3. 流式文本在竖线右侧渲染，与最终 Assistant 消息格式一致
4. 完成后 ProcessingItem 直接变为 AssistantMessageItem，无视觉跳变

---

## 七、Splash 启动页

启动页是整个 app 中猫最"显性"的存在 — 用户看到猫眼闪一下，记住那个 teal 光点，之后进入聊天界面猫就消失了。但 teal 色还在 spinner、角色标签里偶尔出现，像猫在暗处注视。

### 设计哲学

猫不是装饰，是**入口仪式**。启动页 = 猫睁眼，聊天页 = 猫隐身。

### 当前问题

1. 猫眼用 `o` 表示，深色终端下几乎看不见
2. 鼻子 `V` 生硬
3. 标题 `P R I M U S` 字母间距过宽，松散
4. 副标题 `Ready to chat >^.^<` 与上方 ASCII 猫重复表达"猫"
5. 没有动态效果，启动瞬间缺乏仪式感

### 重构方案

#### 静态布局

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

改动点：
- 猫眼 `o` → `◉`（Fisheye），teal 色渲染，像真正的瞳孔在发光
- 猫鼻 `V` → `▾`，更柔和
- 标题 `P R I M U S` → `PRIMUS`，去掉空格，紧凑有力
- 分隔线缩短至 `──── ◆ ────`，居中感更强
- 副标题 `Press Enter`，干净不重复

#### 动态效果：猫眼闪烁

启动后猫眼 `◉` 闪烁一次 teal 光，模拟黑猫在暗处睁眼的瞬间。

**实现**：Splash 组件增加 `blinkCount` 计数器，前 3 帧（约 500ms）猫眼在 `◉` 和 `◦` 之间交替，之后固定为 `◉`。

```
帧 1:  ( ◦   ◦ )    ← 暗
帧 2:  ( ◉   ◉ )    ← 亮（teal 色）
帧 3:  ( ◦   ◦ )    ← 暗
帧 4+: ( ◉   ◉ )    ← 定格亮
```

用 bubbletea 的 `spinner.TickMsg` 驱动，每帧间隔约 160ms。blinkCount 存在 Splash struct 中，`View()` 每次调用时自增。

#### Splash struct 改动

```go
type Splash struct {
    width     int
    height    int
    version   string
    blinkCount int    // 新增：猫眼闪烁计数
}
```

`renderCat()` 中根据 `blinkCount` 决定渲染 `◉` 还是 `◦`：
- `blinkCount < 4 && blinkCount%2 == 0` → `◦`（CatBody 色，暗）
- 否则 → `◉`（CatEye 色，亮）

#### Splash → 聊天页的过渡

用户按下 Enter 后：
1. Splash 消失
2. 如果有 Header（消息 > 0），Header 的猫脸图标 `(=^.^=)` 中眼睛也用 CatEye 色
3. 第一条消息的 Processing spinner 用 teal 色 — 呼应猫眼

整个视觉链路：**猫眼闪烁 → 猫消失 → teal 留在 spinner 和标签里**。用户潜意识里把 teal 色和"猫在看着"关联起来。

---

## 八、Scrollbar 优化

### 问题

当前 scrollbar 用 `┃`/`│` 字符，在窄终端下占用 1 列宝贵空间。

### 方案

**保持当前实现，微调颜色**：

1. Thumb 颜色从 primary (#4ec9b0) 改为 Muted (#a0a0a0)，不抢眼
2. Track 颜色从 subtle (#666666) 改为更暗的 #333333
3. 仅在内容溢出时显示（当前已实现）

不做的事：不改用 Unicode block 元素（▐ █ 等），保持与 box-drawing 字符的风格统一。

---

## 九、Markdown 渲染增强

### 问题

当前 markdown 渲染器比较基础：标题/列表/行内代码/粗体/斜体。代码块只去掉了围栏标记，内容用 subtle 色显示，没有代码块的视觉区分。

### 方案

**代码块加背景暗示**：

1. 围栏代码块内容前加 `│ ` 前缀，用 BorderStyle 渲染，暗示"这是代码区域"
2. 标题（#/##/###）前加 ` ` 空格，用 PrimaryStyle Bold 渲染
3. 列表项的 `•` 符号改为 `·`（更小更轻）
4. 行内代码用反色（深色背景 + 浅色文字）渲染，如果 lipgloss 支持 Background 的话；否则保持当前 SubtleStyle

---

## 十、交互优化

### 10.1 流式输出打断

**问题**：流式输出时所有按键被吞掉（`if m.streaming { return m, nil }`），用户只能干等。

**方案**：Escape 键打断流式输出，保留已生成的部分内容。

```
当前行为:                              重构后:

用户输入 → 等待 → 完成                 用户输入 → 等待 → 按 Esc → 保留已生成内容
  (无法中途停止)                         (立即停止，已输出部分作为 Assistant 消息)
```

实现：
- `tui.go` Update 中，streaming 状态下捕获 `Escape` 键
- 发送 cancel 信号给 LLM 的 stream context
- 将已累积的 `streamText` 作为一条 Assistant 消息添加到列表
- 重置 streaming 状态

### 10.2 斜杠命令反馈

**问题**：`/clear`、`/model` 等命令执行后没有视觉确认，用户不确定是否生效。

**方案**：每个命令返回一条 System 消息确认操作结果。

```
当前:                                  重构后:

用户输入 /clear                        用户输入 /clear
→ 消息清空，无提示                     → 消息清空
                                       → [System] Cleared 4 messages

用户输入 /model                        用户输入 /model
→ 模型切换，无提示                     → [System] Model switched to anthropic/claude-sonnet
```

需修改 `bot.ExecuteCommand()` 的返回值，让每个命令都带确认文本。

### 10.3 发送过渡态

**问题**：按 Enter 到 spinner 出现之间有短暂的"什么都没发生"的感觉。

**方案**：Enter 后立即在 Input 区域显示过渡态。

```
当前:                                  重构后:

┃ Type a message...  ← 输入中          ┃ Type a message...  ← 输入中
  (按 Enter)                            (按 Enter)
  (空白，等待 spinner)                  ┃ ⋯ Sending...       ← 过渡态
┃ ◉ Thinking...      ← spinner 出现    ┃ ◉ Thinking...      ← spinner 出现
```

实现：
- `startChat()` 中先将 Input 的 Prompt 从 `┃` 改为 `⋯`，颜色改为 Muted
- Placeholder 改为 `Sending...`
- spinner 的 TickMsg 到达后恢复正常 Prompt

### 10.4 输入历史翻阅

**问题**：在空输入框里按 ↑ 是在滚动消息区域，但如果用户想翻阅之前发过的消息重新编辑，没有途径。

**方案**：空输入框 + 按 ↑/↓ 时，翻阅发送历史（类似 shell history）。

```
按 ↑:  填入上一条发送过的消息
再按 ↑: 填入更早的消息
按 ↓:  填入下一条消息
按 Esc: 清空输入框，退出历史模式
```

实现：
- `Model` 中新增 `history []string` 和 `historyIdx int`
- `startChat()` 时将消息追加到 history
- 空输入框 + `up` 时，从 history 末尾向前翻阅
- 非空输入框 + `up` 仍保持消息区域滚动行为

### 10.5 Tab 补全

**问题**：输入 `/mo` 后需要手动打完 `/model`，`@agent` 前缀也需要完整输入。

**方案**：Tab 键补全斜杠命令和 @ 前缀。

```
输入 /mo  → Tab → /model
输入 /cl  → Tab → /clear
输入 @ag  → Tab → @agent
```

补全列表：
- `/help`, `/clear`, `/model`, `/config`, `/quit`
- `@agent`

实现：
- 在 Input 的 Update 中捕获 Tab 键
- 匹配当前输入的前缀，找到唯一候选则直接补全
- 多个候选时补全到最长公共前缀

### 10.6 Agent 步骤结构化渲染

**问题**：Agent 模式的步骤输出是纯文本拼接，缺乏视觉结构：

```
当前:
[Step 1] 我来查看文件结构
  Action: 列出目录
  Tool: bash(ls -la)
  Output: total 56
  drwxr-xr-x  5 user user 4096 ...
```

**方案**：用与消息气泡一致的左侧色条格式渲染每个步骤：

```
重构后:
│ Step 1 · 我来查看文件结构
│ Action: 列出目录
│ Tool: bash(ls -la)
│
│ total 56
│ drwxr-xr-x  5 user user 4096 ...
│
│ Step 2 · 分析文件内容
│ ...
```

实现：
- `startAgent()` 中的 stepInfo 拼接改为用 `\n│ ` 前缀
- 每个 step 之间用空行分隔
- Step 标号用 PrimaryStyle 加粗

---

## 十一、实现优先级

| 优先级 | 模块 | 预估改动 | 影响 |
|--------|------|----------|------|
| P0 | 色彩体系 (styles.go) | 小 | 全局观感 |
| P0 | 消息气泡 (types.go) | 中 | 核心体验 |
| P0 | 流式打断 (tui.go) | 小 | 核心交互 |
| P1 | Header 压缩 (header.go) | 中 | 节省空间 |
| P1 | Input 重构 (input.go) | 小 | 底部体验 |
| P1 | 斜杠命令反馈 (bot.go) | 小 | 命令体验 |
| P1 | 发送过渡态 (input.go) | 小 | 流畅感 |
| P2 | Footer 移除/合并 | 小 | 简化布局 |
| P2 | Processing 指示器 | 小 | 流式体验 |
| P2 | 输入历史 (tui.go) | 小 | 效率提升 |
| P2 | Tab 补全 (input.go) | 小 | 效率提升 |
| P2 | Agent 步骤渲染 (tui.go) | 小 | Agent 体验 |
| P3 | Splash 启动页 | 中 | 启动仪式 |
| P3 | Markdown 增强 | 中 | 内容质量 |

建议按 P0 → P1 → P2 → P3 顺序实施。每完成一个优先级可独立验证效果。

---

## 十二、不做的事

- **不用 ANSI 背景色填充整个屏幕** — 保持终端原生背景
- **不在消息区域画猫** — 猫只在启动页和 Header 图标出现
- **不引入新的第三方 TUI 组件库** — 保持 bubbletea + lipgloss 技术栈
- **不做鼠标交互增强** — 当前的滚轮支持已足够
- **不做主题切换** — 深色是唯一主题，黑猫只在夜里出现
