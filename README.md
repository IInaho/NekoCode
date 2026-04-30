<!--
    /\___/\
   ( ◉   ◉ )   PrimusBot
    =  ▾  =
   /|     |\
  (_|     |_)
     || ||
-->

# PrimusBot

终端里的 AI 伙伴 — 聊天、操作文件、执行命令，危险操作会先问你。

### 能做什么

- **自然对话** — 闲聊、问答，自带软萌角色风格
- **文件操作** — 读取、写入、列出目录，写文件前弹框确认
- **Shell 命令** — 执行本地命令，按危险等级自动放行 / 确认 / 拒绝
- **文件搜索** — 模式匹配查找，始终自动放行
- **多 Provider** — 支持 OpenAI / Anthropic / GLM，切换只需改配置
- **长对话记忆** — 自动摘要旧消息，不会超出 token 上限

### 快速开始

```bash
# 1. 配置 API key
mkdir -p ~/.primusbot
cat > ~/.primusbot/config.json << 'EOF'
{
  "provider": "openai",
  "api_key": "sk-your-key-here",
  "model": "gpt-4o",
  "base_url": "https://api.openai.com/v1"
}
EOF

# 2. 构建
go build -o primusbot .

# 3. 交互模式
./primusbot
```

### 使用方式

**交互模式**（TUI）：
```
./primusbot                         # 启动终端界面
```

**命令行模式**（单次调用）：
```bash
./primusbot "帮我看看 main.go 的内容"
./primusbot "列出当前目录所有 Go 文件"
```

**斜杠命令**（交互模式下输入）：

| 命令 | 效果 |
|------|------|
| `/help` | 显示命令列表 |
| `/clear` | 清空对话历史和摘要 |
| `/stats` | 查看上下文状态（消息数 / tokens / 是否有摘要） |
| `/summarize` | 手动触发上下文压缩 |
| `/config` | 查看当前 provider 和 model |

### 权限分级

| 等级 | 行为 | 示例 |
|------|------|------|
| `safe` — 自动放行 | 无需确认，直接执行 | `ls`, `cat`, `find`, `pwd` |
| `write` — 弹框确认 | 终端底部出现确认栏，enter 执行 / esc 取消 | `mkdir`, `cp`, `git commit` |
| `destructive` — 弹框确认 | 同上，标签红色提醒 | `rm`, `kill`, `git push --force` |
| `forbidden` — 直接拒绝 | 不会弹框，聊天中告知被拒 | `sudo`, `curl \| bash`, `ssh` |

### 项目结构

```
primusbot/
├── bot/               # 核心逻辑：Bot 组装 + Agent 循环 + 工具系统
│   ├── agent/          #   Agent 循环（Reason → Execute → Feedback）
│   ├── tools/          #   工具接口 + Bash / FileSystem / Glob
│   ├── bot.go          #   依赖组装
│   ├── command.go       #   斜杠命令
│   └── config.go        #   配置加载
├── ctxmgr/            # 上下文管理：滑窗 + 自动摘要 + token 截断
├── llm/               # LLM 网关：OpenAI / Anthropic / GLM + Function Calling
├── tui/               # 终端 UI：Bubble Tea v2
│   ├── components/     #   消息气泡、输入框、启动页
│   └── styles/         #   色彩体系、Markdown 渲染
├── docs/              # 架构文档 + 设计文档
└── main.go            # 入口
```

### 技术栈

| 层 | 技术 |
|----|------|
| 语言 | Go |
| TUI 框架 | Bubble Tea v2 + Lipgloss |
| LLM 调用 | Native Function Calling（OpenAI / Anthropic / GLM） |
| 上下文管理 | Hybrid Window + LLM Summarization |
| 权限控制 | 四级 DangerLevel + Channel 通信确认 |

### 文档

- [架构文档](docs/ARCHITECTURE.md) — 技术架构、Agent 循环、数据流
- [设计文档](docs/DESIGN.md) — 功能设计、TUI 设计、交互规范

### License

MIT
