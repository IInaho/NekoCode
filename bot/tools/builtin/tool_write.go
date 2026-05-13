package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"nekocode/bot/tools"
)

type WriteTool struct{}

func (t *WriteTool) Name() string                                       { return "write" }
func (t *WriteTool) ExecutionMode(map[string]interface{}) tools.ExecutionMode { return tools.ModeSequential }
func (t *WriteTool) DangerLevel(map[string]interface{}) tools.DangerLevel     { return tools.LevelWrite }
func (t *WriteTool) Description() string {
	return `Create a new file or completely overwrite an existing one.

WHEN TO USE:
- Creating a NEW file (auto-creates parent directories).
- Replacing the ENTIRE content of an existing file.
- For partial modifications, use Edit instead.

REQUIREMENTS:
- If the file ALREADY EXISTS, you MUST Read it first (enforced — the tool will reject writes to unread files).
- NEVER create documentation files (*.md) or README unless explicitly requested.

GOTCHAS:
- Content is a single Go string. Use \n for newlines, \" for quotes, \\ for backslashes.
- Write will overwrite the ENTIRE file — if you only need to change a few lines, use Edit.
- For large files, consider whether Edit would be more efficient.`
}

func (t *WriteTool) Parameters() []tools.Parameter {
	return []tools.Parameter{
		{Name: "path", Type: "string", Required: true, Description: "File path"},
		{Name: "content", Type: "string", Required: true, Description: "Content to write"},
	}
}

func (t *WriteTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	if path == "" {
		return "", fmt.Errorf("missing path parameter")
	}

	safePath, err := tools.ValidatePath(path)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(safePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(safePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}
	return fmt.Sprintf("Written: %s (%d chars)", safePath, len(content)), nil
}
