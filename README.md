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
  <sub>Go · Bubble Tea · OpenAI / Anthropic / GLM · Native Function Calling</sub>
</p>

<br>

<table>
<tr>
<td width="50%"><img src="docs/images/chat.png" width="100%" alt="启动页"></td>
<td width="50%"><img src="docs/images/splash.png" width="100%" alt="聊天界面"></td>
</tr>
</table>

---

### 快速开始

```bash
# 配置 API key
mkdir -p ~/.primusbot
cat > ~/.primusbot/config.json << 'EOF'
{
  "provider": "openai",
  "api_key": "sk-your-key-here",
  "model": "gpt-4o",
  "base_url": "https://api.openai.com/v1"
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

| | | |
|:--|:--|:--|
| **聊天** | 自然对话，自带角色风格 | **Shell** | 执行本地命令 |
| **文件** | 读取、写入、列出目录 | **搜索** | 模式匹配查找文件 |
| **确认** | 写操作弹框确认，危险操作拒绝 | **摘要** | 长对话自动压缩记忆 |
| **Provider** | OpenAI / Anthropic / GLM | **命令** | `/` 斜杠命令 + 实时提示 |

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
| `write` | 底部弹框确认 | `mkdir` `cp` `git commit` |
| `destructive` | 红色标签确认 | `rm` `kill` `git push -f` |
| `forbidden` | 直接拒绝 | `sudo` `curl\|bash` `ssh` |

---

### 结构

```
primusbot/
├── bot/         核心逻辑：Agent 循环 + 工具系统
├── ctxmgr/      上下文：滑窗 + 摘要 + token 截断
├── llm/         LLM 网关：OpenAI / Anthropic / GLM
├── tui/         Bubble Tea v2 终端界面
├── docs/        架构 · 设计 · 路线图
└── main.go      入口
```

---

### 文档

- [架构文档](docs/ARCHITECTURE.md) — Agent 循环 · 数据流 · 上下文管理
- [设计文档](docs/DESIGN.md) — 交互设计 · 视觉主题 · 权限分级
- [开发路线](docs/PLAN.md) — 后续功能计划

---

### License

MIT
