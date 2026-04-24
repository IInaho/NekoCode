# PrimusBot

A terminal AI assistant inspired by OpenCode and Claude Code, built with Go.

## Features

- 💬 Interactive chat interface in terminal
- 🔄 Streaming responses for real-time feedback
- 🤖 Support for OpenAI and Anthropic models
- 📝 Markdown rendering for code and responses
- ⚡ Simple command system for configuration
- 🎨 Clean, colored terminal UI

## Installation

```bash
git clone https://github.com/yourusername/primusbot.git
cd primusbot
go build -o primusbot
```

## Configuration

Create a config file at `~/.primusbot/config.json`:

```json
{
  "provider": "openai",
  "api_key": "your-api-key",
  "model": "gpt-4",
  "base_url": "https://api.openai.com/v1",
  "system_prompt": "You are PrimusBot, a helpful AI assistant...",
  "max_tokens": 4096,
  "temperature": 0.7
}
```

### Supported Providers

- **OpenAI**: Set `provider: "openai"` and provide API key
- **Anthropic**: Set `provider: "anthropic"` and provide API key

## Usage

### Interactive Mode

```bash
./primusbot
```

### Non-Interactive Mode

```bash
./primusbot "Your question here"
```

## Commands

- `/help` - Show help message
- `/clear` - Clear conversation history
- `/model <name>` - Switch model
- `/config` - Show current configuration
- `/quit` - Exit the application

## Architecture

```
primusbot/
├── main.go              # Entry point
├── config/              # Configuration management
├── llm/                 # LLM provider implementations
│   ├── llm.go          # Interface definition
│   ├── openai.go       # OpenAI implementation
│   ├── anthropic.go    # Anthropic implementation
│   └── event_reader.go # SSE stream parser
├── chat/               # Conversation management
├── command/           # Command parsing
└── ui/                # Terminal UI components
```

## Roadmap

- [ ] Enhanced TUI with full bubble tea integration
- [ ] File system operations
- [ ] Code execution sandbox
- [ ] Conversation persistence
- [ ] Multiple conversation sessions
- [ ] Syntax highlighting for code blocks

## License

MIT
