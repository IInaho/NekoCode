# tui — Architecture

Terminal UI for PrimusBot, built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) v2.

## Directory layout

```
tui/
├── tui.go                  # Entry point: Run()
├── model.go                # Model struct, NewModel, Init
├── update.go               # Update() + per-message-type handlers
├── view.go                 # View() layout assembly + cursor positioning
├── stream.go               # StreamState — streaming text buffer with mutex
├── commands.go             # startChat, startAgent, tab completion
├── components/
│   ├── input.go            # Input area (textarea + footer line + borders)
│   ├── messages.go         # Message list (wraps List, manages processing state)
│   ├── header.go           # Top bar (cat icon, provider/model, version)
│   ├── splash.go           # Startup splash (cat + title, centered via lipgloss.Place)
│   ├── message.go          # ChatMessage struct + ToMessageItem factory
│   ├── message_items.go    # Item types: User, Assistant, System, Error
│   ├── processing.go       # ProcessingItem (streaming placeholder)
│   ├── list_widget.go      # Virtual-scrolling list widget
│   └── util.go             # maxInt helper
└── styles/
    ├── charset.go          # Box-drawing chars, Unicode/ASCII fallback
    ├── colors.go           # Color constants, Styles struct, package vars
    └── markdown.go         # Markdown → styled terminal text renderer
```

## Data flow

```
User input (enter)
  → Model.handleKeyPress()
  → Model.startChat(value)
  → Bot.Chat(value, onToken, onDone)
       │
       ├─ onToken → StreamState.Append(content, reasoning)
       │
       └─ onDone → doneMsg{content, reasoning, err}
              → Model.handleDone()
              → Messages.AddMessage(ChatMessage)
```

Streaming rendering is driven by `spinner.TickMsg` (~60fps). Each tick polls `StreamState.Snapshot()` and pushes the current text into `ProcessingItem` inside the message list.

## Key types

| Type | File | Role |
|------|------|------|
| `Model` | model.go | Root Bubble Tea model; owns all components |
| `StreamState` | stream.go | Mutex-guarded text/reasoning buffer for streaming |
| `doneMsg` | model.go | Internal message carrying completed response |
| `Input` | input.go | Textarea wrapper with history, sending state, follow status |
| `Messages` | messages.go | Message list + scrollbar + processing placeholder |
| `List` | list_widget.go | Generic virtual-scrolling list (items: `Item` interface) |
| `ChatMessage` | message.go | Data struct; `ToMessageItem()` converts to list items |
| `Styles` | colors.go | All lipgloss styles; package-level vars (`GreenStyle`, etc.) |

## Component sizes

Each component exposes `Height()` so the layout engine can size the message viewport:

```
+-- Header (2 lines)
+-- Messages (viewport = terminalHeight - header - input - separator)
+-- \n  (2 blank lines for spacing)
+-- Input (5 lines: border + textarea + spacer + footer + border)
```

The formula lives in `update.go:WindowSizeMsg`:
```
contentHeight = msg.Height - Header.Height() - Input.Height() - 2
```

## Cursor positioning

`view.go:View()` manually offsets the textarea cursor. The textarea sits at line 1 of Input (after its top border). The total offset is:

```
cursor.Y = contentLines + 2   // +1 for \n before input, +1 for input's top border
```

Any change to the vertical layout MUST update this offset.

## Streaming lifecycle

1. `startChat()` → `Stream.Start()` + `Spinner.Tick` + async `Bot.Chat()`
2. Each `spinner.TickMsg` → `Stream.Snapshot()` → `Messages.SetStreamText()`
3. `Bot.Chat()` completes → `doneMsg` returned
4. `handleDone()` → `Stream.Stop()` + final `AddMessage()`
5. Escape during streaming → `Bot.CancelStream()` (per-stream context)

## Style system

`styles/colors.go` defines `Styles` struct (~10 lipgloss styles). `DefaultStyles()` creates one instance; package-level vars (`GreenStyle`, `MutedStyle`, etc.) are copies of its fields for convenient direct use.

`styles/charset.go` defines box-drawing constants (`│`, `─`, `┃`, etc.) with ASCII fallback (`|`, `-`) for non-UTF-8 terminals, selected once in `init()`.

`styles/markdown.go` handles inline formatting (`**bold**`, `*italic*`, `` `code` ``), headings (`#`, `##`, `###`), lists (`- `, `* `), and fenced code blocks. Width-aware via `runewidth`.

## Things to know before changing

- **Don't hardcode string widths**: use `lipgloss.Width()` or `runewidth.StringWidth()`. CJK characters are 2 columns wide.
- **Trailing `\n` in View() output** breaks `lipgloss.Height()` (overcounts by 1). The main View() normalizes with `strings.TrimRight(content, "\n")`.
- **List items must implement `Item`**: `Render(width int) string` + `Height(width int) int`. Items cache their rendered output keyed by width.
- **ProcessingItem has a fixed ID**: `"__processing__"`. `SetProcessing(false)` removes it from the list by filtering it out.
- **StreamState is not thread-safe for Stop+Start**: `Stop()` and `Start()` are called on the main goroutine (update loop), `Append()` is called from the stream callback goroutine. The mutex only guards data access, not lifecycle sequencing.
