<!--
    /\___/\
   ( ◉   ◉ )   PrimusBot
    =  ▾  =
   /|     |\
  (_|     |_)
     || ||
-->

<p align="center">
  <br>
  <img src="" width="0" height="0">
</p>

# PrimusBot

<p align="center">
  <b>终端里的 AI 伙伴</b><br>
  <sub>软萌猫娘角色 · 能聊天、操作文件、执行命令 · 持续迭代中</sub>
</p>

<p align="center">
  <sub>Go · Bubble Tea · OpenAI / Anthropic / GLM / DeepSeek · Native Function Calling</sub>
</p>

<br>

<table>
<tr>
<td width="50%"><img src="docs/images/splash.png" width="100%" alt="启动页"></td>
<td width="50%"><img src="docs/images/chat.png" width="100%" alt="聊天界面"></td>
</tr>
</table>

---

### 亮点

**🎨 精心打磨的 TUI 体验**
- **Plan B 厚左色条**：`▐` 粗块 + 角色专属配色（金/青/蓝/红），lipgloss 块级渲染，UI 与内容零 ANSI 混拼
- **工具卡片**：暖金色边框独立卡片，可折叠，多工具聚合在单一卡片
- **独立 Scrollbar**：自管理渲染，与消息列表并列排版，无宽度冲突
- **💭 思考过程**：实时展示 Agent 推理链，工具调用与思考文本穿插
- tokoyo-night markdown 主题 + glamour 增量渲染

**⚡ 轻量高效的 Agent 循环**
- Reason → Execute → Feedback 三轮循环，最多 15 步
- 并行工具调度：独立工具（glob+grep+web_search）worker pool 并发，任一 Sequential 降级串行
- ShouldStop 策略：搜索 4 轮无 fetch → 强制综合；工具结果 > 6 → 注入综合指令
- 上下文 Hybrid Window + Summary：滑窗 + 自动摘要 + 语言感知 token 估算

**🔧 丰富的工具链**
- bash（四级危险分级 + ANSI 清理）、filesystem（SHA256 去重）、glob、grep、edit（+ diff）
- web_search（Bing HTML 解析 + CJK 排序）、web_fetch（DNS 安全校验）

**🔌 多 Provider 统一网关**
- Anthropic：SSE content_block_start/delta 流式解析，tool_use 双向转换
- OpenAI / GLM / DeepSeek：统一 OpenAICompatible 实现，共享 HTTP 连接池
- ReasoningContent 透传 + 增量 token 统计

**🧩 组件化解耦**
- `BotInterface` 14 方法接口，TUI 与 bot 零耦合
- `phase.go` 独立状态模块：Ready → Thinking → Reasoning → Running
- ctxmgr 4 文件拆分 + tui 21 组件文件 + llm 4 provider 文件

---

### 快速开始

```bash
mkdir -p ~/.primusbot
cat > ~/.primusbot/config.json << 'EOF'
{
  "provider": "anthropic",
  "api_key": "sk-your-key-here",
  "model": "claude-sonnet-4-5",
  "base_url": "https://api.anthropic.com/v1",
  "token_budget": 128000
}
EOF

go build -o primusbot .

# 交互模式
./primusbot

# 或单次调用
./primusbot "帮我看看 main.go 的内容"
```

---

### 功能

| | | | |
|:--|:--|:--|:--|
| **聊天** | 自然对话，软萌猫娘角色 | **Shell** | 本地命令，四级权限分级 |
| **文件** | 读取、写入、列出目录 | **搜索** | glob 模式 + ripgrep + web |
| **编辑** | 精确字符串替换 + diff | **摘要** | 长对话自动压缩记忆 |
| **确认** | 写/危险操作弹框确认 | **命令** | `/` 斜杠命令 + 实时提示 |
| **折叠** | `ctrl+e` 展开工具卡片 | **Provider** | OpenAI / Anth / GLM / DS |

---

### 命令

| 命令 | |
|------|------|
| `/help` | 显示命令列表 |
| `/clear` | 清空对话历史 |
| `/stats` | 上下文用量 |
| `/summarize` | 手动压缩记忆 |
| `/config` | 当前 provider/model |

输入 `/` 自动弹出提示，Tab 选择，Enter 填入。

---

### 权限

| 等级 | 行为 | 示例 |
|:--|:--|:--|
| `safe` | 自动放行 | `ls` `cat` `find` `pwd` |
| `write` | 弹框确认 | `mkdir` `cp` `git commit` |
| `destructive` | 红色确认 | `rm` `kill` `git push -f` |
| `forbidden` | 直接拒绝 | `sudo` `curl\|bash` `ssh` |

---

### 结构

```
primusbot/
├── bot/         核心逻辑：Agent 循环 + 工具系统 + 扩展 + 类型
├── ctxmgr/      上下文管理：滑窗 + 摘要 + token 估算（4 文件）
├── llm/         LLM 网关：Anthropic / OpenAI 兼容（4 文件）
├── tui/         Bubble Tea v2 终端界面，BotInterface 解耦（21 文件）
├── docs/        架构 · 设计 · 路线图
└── main.go      入口
```

---

### 文档

- [架构文档](docs/ARCHITECTURE.md) — Agent 循环 · 数据流 · 上下文管理 · 组件树
- [设计文档](docs/DESIGN.md) — 交互设计 · 视觉方案 · 权限分级 · ContentBlock
- [开发路线](docs/PLAN.md) — 已完成 & 后续计划

---

### License

MIT
