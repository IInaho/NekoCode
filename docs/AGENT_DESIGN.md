# PrimusBot Agent 循环模式设计方案

## 一、项目现状分析

### 1.1 当前架构
```
用户输入 → 命令解析(/命令) → LLM流式响应 → UI展示
```

### 1.2 现有模块
| 模块 | 职责 |
|------|------|
| bot/bot.go | 核心Bot，整合ChatManager、LLM、CommandParser |
| chat/chat.go | 消息历史管理 |
| llm/* | LLM接口抽象（OpenAI/Anthropic/GLM） |
| command/parser.go | 命令解析（/help、/clear、/model等） |
| ui/tui.go | Bubble Tea TUI界面 |

### 1.3 当前局限
- 仅支持被动响应，无自主行动能力
- 无工具调用机制
- 无多轮推理能力
- 无状态跟踪和目标管理

---

## 二、Agent 循环模式设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        User Input                            │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  感知层 (Perception)                                         │
│  - 输入解析（命令/普通消息）                                    │
│  - 上下文构建（历史消息+系统状态）                               │
│  - 环境信息收集（时间、配置、状态）                              │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  决策层 (Reasoning)                                          │
│  - LLM推理（判断意图、决定行动）                                │
│  - 行动计划生成                                               │
│  - 循环控制（继续/停止）                                       │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  行动层 (Action)                                             │
│  - 工具执行（文件系统、网络请求、命令执行）                       │
│  - 响应生成（文本/流式输出）                                   │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│  反馈层 (Feedback)                                           │
│  - UI状态更新                                                │
│  - 历史记录更新                                              │
│  - 错误处理与重试                                             │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 核心组件设计

#### 2.2.1 Agent 核心结构

```go
// agent/agent.go
package agent

type Agent struct {
    ctx           context.Context
    chatManager   *chat.ChatManager
    llmClient     llm.LLM
    toolRegistry  *ToolRegistry
    memory        *Memory
    config        *AgentConfig
    
    // 循环控制
    maxIterations int
    currentStep   int
}

type AgentConfig struct {
    MaxIterations int           // 最大迭代次数 (默认10)
    Timeout       time.Duration // 单次行动超时
    EnableTools   bool          // 是否启用工具
    SystemPrompt  string        // Agent专用提示
}

type Memory struct {
    // 工作记忆：当前任务相关
    workingMemory map[string]interface{}
    // 持久记忆：跨会话信息
    persistentMemory []MemoryItem
}
```

#### 2.2.2 工具系统

```go
// agent/tools/tool.go
package tools

type Tool interface {
    Name() string
    Description() string
    Parameters() []Parameter
    Execute(ctx context.Context, args map[string]interface{}) (string, error)
}

type Parameter struct {
    Name        string
    Type        string
    Required    bool
    Description string
}

type ToolResult struct {
    Success bool
    Output  string
    Error   string
    Metadata map[string]interface{}
}
```

#### 2.2.3 推理循环

```go
// agent/reasoner.go
package agent

type ReasoningStep struct {
    Thought     string  // 思考过程
    Action      string  // 执行动作
    ActionInput string  // 动作输入
    Observation string  // 观察结果
}

func (a *Agent) Run(input string) (<-chan string, <-chan error) {
    // 1. 感知
    state := a.perceive(input)
    
    // 2. 决策循环
    for a.currentStep < a.maxIterations {
        // 推理
        step := a.reason(state)
        
        // 行动
        result := a.act(step)
        
        // 反馈
        state = a.feedback(state, result)
        
        // 检查是否完成
        if step.IsFinal {
            break
        }
    }
    
    // 返回最终结果
}
```

---

## 三、具体实现方案

### 3.1 目录结构

```
agent/
├── agent.go          # Agent核心
├── config.go         # 配置
├── perceive.go       # 感知模块
├── reasoner.go       # 推理模块
├── executor.go       # 执行模块
├── feedback.go       # 反馈模块
├── memory.go         # 记忆系统
└── tools/
    ├── tool.go       # 工具接口
    ├── filesystem.go # 文件操作
    ├── exec.go       # 命令执行
    ├── http.go       # HTTP请求
    └── registry.go  # 工具注册
```

### 3.2 感知层实现

```go
// agent/perceive.go
package agent

type PerceptionResult struct {
    InputType    InputType      // command/text
    Intent       string         // 识别的意图
    Entities     map[string]interface{}  // 提取的实体
    Context      map[string]interface{}  // 上下文信息
    RawInput     string
}

type InputType int
const (
    InputTypeCommand InputType = iota
    InputTypeText
    InputTypeAgentTask
)

func (a *Agent) perceive(input string) *PerceptionResult {
    // 1. 检查是否为命令
    if strings.HasPrefix(input, "/") {
        cmd := a.cmdParser.Parse(input)
        if cmd.Name != "" {
            return &PerceptionResult{
                InputType: InputTypeCommand,
                Intent:   "execute_command",
                Entities: map[string]interface{}{
                    "command": cmd.Name,
                    "args":    cmd.Args,
                },
                RawInput: input,
            }
        }
    }
    
    // 2. 检查是否为Agent任务（以@agent开头）
    if strings.HasPrefix(input, "@agent") {
        return &PerceptionResult{
            InputType: InputTypeAgentTask,
            Intent:   "agent_task",
            Entities: map[string]interface{}{
                "task": strings.TrimPrefix(input, "@agent"),
            },
            RawInput: input,
        }
    }
    
    // 3. 普通对话
    return &PerceptionResult{
        InputType: InputTypeText,
        Intent:   "chat",
        Context: map[string]interface{}{
            "timestamp": time.Now(),
            "message_count": a.chatManager.MessageCount(),
        },
        RawInput: input,
    }
}
```

### 3.3 决策层实现

```go
// agent/reasoner.go
package agent

type ReasoningResult struct {
    Thought       string
    Action        ActionType
    ActionInput   string
    ShouldContinue bool
    IsFinal       bool
}

type ActionType int
const (
    ActionChat ActionType = iota
    ActionExecuteTool
    ActionFinish
    ActionAskClarification
)

func (a *Agent) reason(state *PerceptionResult) *ReasoningResult {
    // 构建推理上下文
    context := a.buildReasoningContext(state)
    
    // 调用LLM进行推理
    prompt := a.buildReasoningPrompt(context)
    response, err := a.llmClient.Chat(a.ctx, []llm.Message{
        {Role: "user", Content: prompt},
    })
    
    if err != nil {
        return &ReasoningResult{
            Thought: fmt.Sprintf("推理失败: %v", err),
            Action:  ActionFinish,
            IsFinal: true,
        }
    }
    
    // 解析LLM响应
    return a.parseReasoningResponse(response.Choices[0].Message.Content)
}

func (a *Agent) buildReasoningPrompt(context *ReasoningContext) string {
    return fmt.Sprintf(`
当前状态：
- 输入类型：%s
- 意图：%s
- 可用工具：%s

历史消息：
%s

请决定下一步行动。格式：
Thought: <你的思考>
Action: <execute_tool|chat|finish>
ActionInput: <工具名:参数 或 消息内容>
`, context.InputType, context.Intent, 
    a.toolRegistry.AvailableToolsString(),
    context.HistorySummary)
}
```

### 3.4 行动层实现

```go
// agent/executor.go
package agent

func (a *Agent) act(reasoning *ReasoningResult) *ActionResult {
    switch reasoning.Action {
    case ActionExecuteTool:
        return a.executeTool(reasoning.ActionInput)
    case ActionChat:
        return a.generateChatResponse(reasoning.ActionInput)
    case ActionFinish:
        return &ActionResult{Success: true, Output: reasoning.ActionInput, IsFinal: true}
    default:
        return &ActionResult{Success: false, Error: "未知行动类型"}
    }
}

func (a *Agent) executeTool(input string) *ActionResult {
    parts := strings.SplitN(input, ":", 2)
    if len(parts) != 2 {
        return &ActionResult{Success: false, Error: "工具调用格式错误"}
    }
    
    toolName := parts[0]
    args := parts[1]
    
    tool := a.toolRegistry.Get(toolName)
    if tool == nil {
        return &ActionResult{Success: false, Error: fmt.Sprintf("工具不存在: %s", toolName)}
    }
    
    // 解析参数
    parsedArgs := parseArguments(args)
    
    // 执行
    result, err := tool.Execute(a.ctx, parsedArgs)
    if err != nil {
        return &ActionResult{Success: false, Error: err.Error()}
    }
    
    return &ActionResult{
        Success: true,
        Output:  result,
    }
}
```

### 3.5 反馈层实现

```go
// agent/feedback.go
package agent

type FeedbackResult struct {
    UpdatedState *PerceptionResult
    UIUpdate     UIUpdate
    ShouldRetry  bool
}

type UIUpdate struct {
    MessageType  string  // user/assistant/system/error
    Content      string
    Stream       bool    // 是否流式输出
}

func (a *Agent) feedback(state *PerceptionResult, result *ActionResult) *PerceptionResult {
    // 1. 更新记忆
    a.memory.Add(MemoryItem{
        Step:     a.currentStep,
        Thought:  result.Thought,
        Action:   result.Action,
        Output:   result.Output,
        Timestamp: time.Now(),
    })
    
    // 2. 构建新的状态
    newState := &PerceptionResult{
        InputType: InputTypeText,
        Intent:   "observation",
        Entities: map[string]interface{}{
            "previous_action": result.Action,
            "previous_output": result.Output,
            "success":         result.Success,
        },
        Context: state.Context,
    }
    
    // 3. 判断是否需要重试
    if !result.Success && result.ShouldRetry {
        newState.Context["retry_count"] = state.Context["retry_count"].(int) + 1
    }
    
    return newState
}
```

---

## 四、工具系统设计

### 4.1 内置工具

```go
// agent/tools/filesystem.go
type FileSystemTool struct{}

func (t *FileSystemTool) Name() string { return "filesystem" }
func (t *FileSystemTool) Description() string { 
    return "读取或写入文件，列出目录" 
}

func (t *FileSystemTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    operation := args["operation"].(string)
    path := args["path"].(string)
    
    switch operation {
    case "read":
        content, err := os.ReadFile(path)
        return string(content), err
    case "write":
        content := args["content"].(string)
        return "写入成功", os.WriteFile(path, []byte(content), 0644)
    case "list":
        entries, err := os.ReadDir(path)
        // ...处理
    }
    return "", nil
}

// agent/tools/exec.go
type ExecTool struct{}

func (t *ExecTool) Name() string { return "exec" }
func (t *ExecTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
    cmd := args["command"].(string)
    parts := strings.Fields(cmd)
    
    proc := exec.CommandContext(ctx, parts[0], parts[1:]...)
    output, err := proc.CombinedOutput()
    
    return string(output), err
}
```

### 4.2 工具注册

```go
// agent/tools/registry.go
type ToolRegistry struct {
    tools map[string]Tool
    mu    sync.RWMutex
}

func (r *ToolRegistry) Register(tool Tool) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.tools[tool.Name()] = tool
}

func (r *ToolRegistry) Get(name string) Tool {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.tools[name]
}

// 注册默认工具
func RegisterDefaultTools(registry *ToolRegistry) {
    registry.Register(&FileSystemTool{})
    registry.Register(&ExecTool{})
    registry.Register(&HTTPClientTool{})
}
```

---

## 五、LLM 提示词设计

### 5.1 Agent 系统提示

```markdown
你是一个智能助手，可以执行各种任务和操作。

## 可用能力
1. **对话**：与用户进行自然语言交互
2. **工具调用**：在需要时调用内置工具完成特定任务

## 工具列表
- `filesystem`: 文件系统操作（读、写、列表）
- `exec`: 执行Shell命令
- `http`: 发送HTTP请求

## 推理模式
当你需要完成复杂任务时，请按以下格式思考：

```
Thought: 分析当前情况，确定需要做什么
Action: 选择下一步行动（execute_tool/chat/finish）
ActionInput: 具体执行内容
Observation: 执行结果
```

## 规则
1. 如果不确定工具参数，先询问用户
2. 执行危险操作前提示用户确认
3. 始终保持友好和帮助性
```

---

## 六、循环终止条件

| 条件 | 说明 |
|------|------|
| `ActionFinish` | LLM判断任务已完成 |
| 达到`maxIterations` | 防止无限循环（默认10次） |
| 超时 | 单次执行超时（默认60s） |
| 用户中断 | Ctrl+C退出 |
| 错误重试耗尽 | 连续失败3次后停止 |

---

## 七、兼容性设计

### 7.1 向后兼容
- 原有的`/命令`继续有效
- 普通聊天仍然直接调用LLM
- 新增`@agent`前缀启用Agent模式

### 7.2 配置选项
```json
{
  "agent": {
    "enabled": true,
    "max_iterations": 10,
    "timeout_seconds": 60,
    "default_tools": ["filesystem", "exec"]
  }
}
```

---

## 八、实施建议

### 阶段一：基础框架
1. 创建 `agent/` 目录和基础结构
2. 实现感知-决策-行动-反馈循环骨架
3. 集成现有LLM调用

### 阶段二：工具系统
1. 实现工具接口和注册机制
2. 添加基础工具（filesystem, exec）
3. 扩展HTTP工具

### 阶段三：增强功能
1. 添加记忆系统
2. 优化提示词提升推理质量
3. 添加流式输出支持