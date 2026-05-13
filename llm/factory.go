package llm

// NewClient creates an LLM client configured for the given provider and model.
// thinkingBudget is only used for Anthropic; OpenAI/DeepSeek use reasoning_effort
// or disable thinking entirely (auto-max can't be controlled).
func NewClient(provider, apiKey, baseURL, model string, thinkingBudget int) LLM {
	var c LLM
	switch provider {
	case "anthropic":
		ac := NewAnthropic(apiKey, baseURL, model)
		ac.SetThinkingBudget(thinkingBudget)
		c = ac
	case "glm":
		gc := NewGLM(apiKey, baseURL, model)
		gc.SetDisableThinking(true)
		c = gc
	default:
		oc := NewOpenAI(apiKey, baseURL, model)
		oc.SetDisableThinking(true)
		c = oc
	}
	return c
}

// Clone creates a fresh LLM client for the given provider/model.
// Matches NewClient defaults: thinking disabled for non-Anthropic providers.
func Clone(provider, apiKey, baseURL, model string, thinkingBudget int) LLM {
	switch provider {
	case "anthropic":
		c := NewAnthropic(apiKey, baseURL, model)
		c.SetThinkingBudget(thinkingBudget)
		return c
	case "glm":
		c := NewGLM(apiKey, baseURL, model)
		c.SetDisableThinking(true)
		return c
	default:
		c := NewOpenAI(apiKey, baseURL, model)
		c.SetDisableThinking(true)
		return c
	}
}
