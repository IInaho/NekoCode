package command

import (
	"strings"
)

type Command struct {
	Name string
	Args []string
	Raw  string
}

type CommandHandler func(cmd *Command) (string, bool)

type Parser struct {
	handlers map[string]CommandHandler
}

func NewParser() *Parser {
	return &Parser{
		handlers: make(map[string]CommandHandler),
	}
}

func (p *Parser) Register(name string, handler CommandHandler) {
	p.handlers[name] = handler
}

func (p *Parser) Parse(input string) *Command {
	trimmed := strings.TrimSpace(input)

	if !strings.HasPrefix(trimmed, "/") {
		return &Command{
			Name: "",
			Raw:  input,
		}
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return &Command{
			Name: "",
			Raw:  input,
		}
	}

	name := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}

	remaining := trimmed[len(parts[0]):]
	if len(remaining) > 0 {
		remaining = strings.TrimSpace(remaining[len(parts[0]):])
		if remaining != "" && len(args) == 0 {
		}
	}

	return &Command{
		Name: name,
		Args: args,
		Raw:  input,
	}
}

func (p *Parser) Execute(cmd *Command) (string, bool) {
	if cmd.Name == "" {
		return "", false
	}

	handler, exists := p.handlers[cmd.Name]
	if !exists {
		return "", true
	}

	return handler(cmd)
}

func RegisterDefaultCommands(p *Parser, callbacks *CommandCallbacks) {
	p.Register("help", func(cmd *Command) (string, bool) {
		return `Available commands:
  /help          Show this help message
  /clear         Clear conversation history
  /model <name>  Switch to a different model
  /config        Show current configuration
  /quit          Exit the application
  /<text>        Send a message to the AI
`, true
	})

	p.Register("clear", func(cmd *Command) (string, bool) {
		if callbacks.ClearHistory != nil {
			callbacks.ClearHistory()
		}
		return "Conversation history cleared.", true
	})

	p.Register("quit", func(cmd *Command) (string, bool) {
		if callbacks.Quit != nil {
			callbacks.Quit()
		}
		return "", true
	})

	p.Register("exit", func(cmd *Command) (string, bool) {
		if callbacks.Quit != nil {
			callbacks.Quit()
		}
		return "", true
	})

	p.Register("config", func(cmd *Command) (string, bool) {
		if callbacks.GetConfig != nil {
			return callbacks.GetConfig(), true
		}
		return "", true
	})

	p.Register("model", func(cmd *Command) (string, bool) {
		if len(cmd.Args) > 0 && callbacks.SetModel != nil {
			callbacks.SetModel(cmd.Args[0])
			return "Model switched to " + cmd.Args[0], true
		}
		return "Usage: /model <model-name>", true
	})
}

type CommandCallbacks struct {
	ClearHistory func()
	Quit         func()
	GetConfig    func() string
	SetModel     func(model string)
}

func Parse(input string) *Command {
	trimmed := strings.TrimSpace(input)

	if !strings.HasPrefix(trimmed, "/") {
		return &Command{
			Name: "",
			Raw:  input,
		}
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return &Command{
			Name: "",
			Raw:  input,
		}
	}

	name := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}

	return &Command{
		Name: name,
		Args: args,
		Raw:  input,
	}
}
