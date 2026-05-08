package subagent

// AgentType defines a built-in sub-agent: its identity, constraints, and behavior.
type AgentType struct {
	Name         string   // unique identifier
	Description  string   // when to use (injected into system prompt for LLM selection)
	SystemPrompt string   // the sub-agent's role + rules
	Tools        []string // allowed tool names; nil = all tools
	MaxSteps     int      // max reasoning steps
	TokenBudget  int      // max estimated tokens before auto-compaction
}

// RunConfig is the input contract for the engine.
type RunConfig struct {
	Prompt          string             // the sub-agent's task description
	AgentType       AgentType          // which agent profile to use
	Cwd             string             // working directory
	OnPhase         func(phase string) // optional: phase changes (Thinking/Running/done)
	AddTokens       func(prompt, compl int) // optional: token usage callback
	DisableThinking bool               // append conciseness directive for sub-agents
}

var builtins = map[string]AgentType{}

func register(a AgentType) {
	builtins[a.Name] = a
}

// Get looks up a built-in agent type by name.
func Get(name string) (AgentType, bool) {
	a, ok := builtins[name]
	return a, ok
}

// List returns all registered built-in agent types.
func List() []AgentType {
	out := make([]AgentType, 0, len(builtins))
	for _, a := range builtins {
		out = append(out, a)
	}
	return out
}
