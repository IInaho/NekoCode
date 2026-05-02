# P2 18 — Plan 模式技术方案

## 调研摘要

分析了 6 个主流 code agent 的 plan 模式实现：

| Agent | 核心机制 | 工具限制 | 计划格式 | 模型路由 |
|-------|---------|---------|---------|---------|
| **Claude Code** | 独立权限模式，`EnterPlanMode`/`ExitPlanMode` 工具 | 只读工具可用，写入需逐步骤审批 | 结构化 Markdown，ExitPlanMode 时呈现审批 | Opus(规划)→Sonnet(执行) |
| **Cline** | `Mode = "plan" \| "act"` 双模式，系统提示注入 `ACT_VS_PLAN_SECTION` | plan: `plan_mode_respond` + 只读; act: 全部工具 | Markdown，通过专用 `plan_mode_respond` 工具强制输出 | 无（按 variant 注册工具） |
| **Aider** | Architect/Editor 双模型流水线 | 架构师提出方案，编辑器执行 | 纯文本，架构师输出直接喂给编辑器 | 主模型(architect) + `--editor-model`(editor) |
| **Cursor** | Plan Mission Control | 规范生成阶段只读，Agent 模式全权限 | YAML-ish 规范(触发器/行为/任务) | 多模型后端 |
| **OpenHands** | 无独立 plan 模式，通过对话隐式规划 | 统一的 CodeAct 动作空间 | 无形式化计划工件 | 无 |
| **通用模式** | 专用系统提示 + 工具集裁剪为只读 + 用户审批切换 | 只读: read/search/web; 禁用: write/edit/bash | Markdown 为主，结构化章节 | 大模型规划 + 小模型执行 |

### 关键洞察

1. **系统提示注入是核心**：所有实现都通过注入 plan 模式专用提示来改变 LLM 行为，从"执行者"切换为"规划者"
2. **工具集裁剪**：plan 模式只开放只读工具（read/search/grep/glob），禁止写入和 shell 执行
3. **审批是必备环节**：plan→execution 的切换必须经过用户审批，Claude Code 提供 5 种审批选项
4. **上下文管理**：切换时多数实现会清理/压缩 plan 模式的探索上下文，只保留计划文本进入执行阶段
5. **模型路由是可选的优化**：Claude Code 和 Aider 做了，Cline 没做。对 PrimusBot 现阶段非必需

---

## PrimusBot Plan 模式设计

### 架构概览

```
用户输入 "实现 XXX 功能"
        │
        ▼
┌──────────────────┐
│  /plan 命令       │  ← 或自动检测复杂度
│  进入 plan 模式   │
└──────┬───────────┘
       │
       ▼
┌──────────────────┐
│  Plan 模式循环    │  ← 只读工具 + Plan 系统提示
│  explore → plan  │
└──────┬───────────┘
       │ 产出 plan markdown
       ▼
┌──────────────────┐
│  审批卡片         │  ← TUI 渲染计划 + 选项
│  [批准][修改][取消]│
└──────┬───────────┘
       │ 批准
       ▼
┌──────────────────┐
│  Execute 模式     │  ← 全部工具 + 计划注入上下文
│  执行计划         │
└──────────────────┘
```

### 1. 模式定义

在 `bot/agent/agent.go` 中新增模式枚举：

```go
type AgentMode int
const (
    ModeDefault AgentMode = iota  // 正常模式，全部工具
    ModePlan                      // plan 模式，只读工具
    ModeExecute                   // 执行模式，全部工具 + 计划注入
)
```

### 2. 系统提示切换

在 `bot/prompt/` 下新增 `plan.md`：

```
你是 PrimusBot 的架构师模式。你的职责是：
1. 深入理解用户需求，探索相关代码
2. 分析现状，提出可行的技术方案
3. 产出结构化的实施计划

规则：
- 你可以使用 glob/grep/filesystem(read) 探索代码
- 你可以使用 bash 运行只读命令（git log、cat、ls 等）
- 你不能修改任何文件
- 当你准备好方案时，用自然语言输出一个结构化的计划

计划格式（Markdown）：
## 分析
## 方案
## 涉及文件
## 实施步骤
## 风险与注意事项
```

**实现**：`ctxmgr.Manager` 新增 `SetSystemPrompt(prompt string)` 方法，切换模式时替换。

### 3. 工具集裁剪

Plan 模式下只暴露 LevelSafe 工具：

```go
// bot/tools/tool.go - Registry 新增方法
func (r *Registry) DescriptorsByLevel(maxLevel DangerLevel) []Descriptor {
    // 只返回 DangerLevel <= maxLevel 的工具
}
```

Plan 模式工具集：

| 工具 | 状态 | 备注 |
|------|------|------|
| `glob` | ✅ 开放 | LevelSafe |
| `grep` | ✅ 开放 | LevelSafe |
| `filesystem` | ⚠️ 仅 read/list | 需要子操作级别过滤 |
| `bash` | ⚠️ 受限 | 仅允许只读命令，或替换为 `bash_readonly` |
| `edit` | ❌ 禁用 | LevelWrite |

**bash 受限方案**：新增 `bash_readonly` 工具，包装 bash 但只允许白名单命令（`git log/diff/show/status`、`cat`、`ls`、`find`、`wc`、`head`、`tail` 等），或通过 `DangerLevel` 动态判断。

### 4. Agent 循环改造

在 `bot/agent/run.go` 的 `Agent.Run()` 中增加模式分支：

```go
func (a *Agent) Run(input string, callback RunCallback) *RunResult {
    if a.mode == ModePlan {
        return a.runPlanLoop(input, callback)
    }
    return a.runDefaultLoop(input, callback)
}
```

**Plan 循环**（`runPlanLoop`）：
- 与默认循环相同（Reason → Execute → Feedback）
- 差异仅在：系统提示不同 + 工具集不同
- 当 LLM 输出 `ActionChat`（文本回复）且内容包含计划格式时，视为计划产出
- 将计划文本通过 `PlanProposal` 返回给 TUI

### 5. 审批流程

**TUI 端**：新增 `PlanApprovalCard` 组件，类似 `ConfirmBar`：

```
┌─────────────────────────────────────────┐
│  📋 实施计划                              │
│                                          │
│  ## 分析                                  │
│  当前 XXX 模块缺少 YYY 功能...             │
│                                          │
│  ## 方案                                  │
│  采用 ZZZ 模式，在 A/B/C 文件中...         │
│                                          │
│  ## 涉及文件                              │
│  - src/foo/bar.go                        │
│  - src/foo/baz.go                        │
│                                          │
│  ## 实施步骤                              │
│  1. 重构 bar.go 的 HandleXXX             │
│  2. 在 baz.go 新增 ProcessYYY            │
│  3. 注册路由                              │
│                                          │
│  [批准并执行] [修改计划] [取消]            │
└─────────────────────────────────────────┘
```

**BotInterface 扩展**：

```go
type PlanProposal struct {
    Content string   // 计划 Markdown 文本
    Files   []string // 涉及的文件列表
    Steps   []string // 实施步骤
}

type BotInterface interface {
    // ... 现有方法 ...
    SetPlanApproveFn(fn func(proposal PlanProposal) PlanAction)
}
```

### 6. 上下文管理

Plan → Execute 切换时的上下文策略（参考 Aider 的做法）：

```
Plan 阶段消息 ──→ 压缩为摘要 ──→ 注入 Execute 阶段 system prompt
Plan 文本      ──→ 作为 user message ──→ 注入 Execute 阶段消息列表
```

**实现**：
1. Plan 阶段结束后，调用 `ctxMgr.Summarize()` 压缩探索消息
2. 将 plan 文本作为 `user` 角色消息追加：`"以下是已批准的实施计划，请按照计划逐步执行：\n\n{plan}"`
3. 切换到 Execute 模式（全工具集 + 默认 system prompt）

### 7. 入口方式

两种触发 plan 模式的方式：

| 方式 | 实现 | 优先级 |
|------|------|--------|
| `/plan` 命令 | 用户显式进入 plan 模式 | P0 |
| 自动检测 | `Reason()` 阶段判断任务复杂度，提示用户是否进入 plan 模式 | P1 |

`/plan` 命令注册在 `cmdParser` 中，调用 `Bot.EnterPlanMode()`。

### 8. 任务列表生成（可选 P1）

从审批通过的计划中自动提取步骤，创建 Todo 列表（与 P2 14 Todo tracking 联动）：

```
计划步骤 ──→ 解析 ──→ TaskCreate ──→ TUI 实时显示进度
```

---

## 实施步骤

### Phase 1 — 核心骨架（预计 4-6h）

1. **`bot/agent/agent.go`**：新增 `AgentMode` 枚举和 `SetMode()` 方法
2. **`bot/agent/run.go`**：拆分 `runPlanLoop` / `runDefaultLoop`
3. **`bot/tools/tool.go`**：`Registry.DescriptorsByLevel()` 方法
4. **`bot/prompt/plan.md`**：Plan 模式系统提示
5. **`ctxmgr/manager.go`**：`SetSystemPrompt()` 方法

### Phase 2 — TUI 审批（预计 3-4h）

6. **`bot/types/types.go`**：新增 `PlanProposal`、`PlanAction` 类型
7. **`tui/components/plan_card.go`**：审批卡片组件
8. **`tui/model.go`**：新增 `PlanApproveFn` 注册和流程编排

### Phase 3 — 联调与优化（预计 2-3h）

9. **`bot/bot.go`**：`/plan` 命令注册，模式切换入口
10. bash 只读模式 / filesystem 子操作过滤
11. 上下文压缩与计划注入
12. 端到端测试

---

## 风险与取舍

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 模型路由 | 不做 | PrimusBot 用户通常只有一个 provider，做模型切换增加配置复杂度 |
| 计划格式校验 | 不做 | 依赖 LLM 输出质量 + 用户审批兜底，不引入形式化 schema |
| 自动复杂度检测 | P1 | 初期 `/plan` 命令够用，自动检测需要额外的判断逻辑 |
| Plan 探索上下文 | 压缩不丢弃 | Aider 的丢弃策略简单但丢失上下文，压缩保留关键信息 |
