package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type WriteTool struct{}

func (t *WriteTool) Name() string                                       { return "write" }
func (t *WriteTool) ExecutionMode(map[string]interface{}) ExecutionMode { return ModeSequential }
func (t *WriteTool) DangerLevel(map[string]interface{}) DangerLevel     { return LevelWrite }
func (t *WriteTool) Description() string {
	return "Create or overwrite a file. MUST Read existing file first (or will fail). Prefer Edit for modifications. NEVER create documentation files (*.md) or README. Auto-creates parent directories."
}

func (t *WriteTool) Parameters() []Parameter {
	return []Parameter{
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

	safePath, err := validatePath(path)
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
