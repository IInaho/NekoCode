package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontmatter is the YAML header of a SKILL.md file.
// Field names follow the Claude Code convention, which is the de facto standard.
type frontmatter struct {
	Name                   string   `yaml:"name"`
	Description            string   `yaml:"description"`
	WhenToUse              string   `yaml:"when_to_use"`
	AllowedTools           []string `yaml:"allowed-tools"`
	Context                string   `yaml:"context"` // "inline" or "fork"
	Agent                  string   `yaml:"agent"`
	MaxSteps               int      `yaml:"max_steps"`
	TokenBudget            int      `yaml:"token_budget"`
	Paths                  []string `yaml:"paths"`
	DisableModelInvocation bool     `yaml:"disable-model-invocation"`
	UserInvocable          bool     `yaml:"user-invocable"`
	ArgumentHint           string   `yaml:"argument-hint"`
}

// loadSkill parses a SKILL.md file and returns a Skill.
func loadSkill(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	fm, body, err := parseFrontmatter(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	if fm.Name == "" {
		return nil, fmt.Errorf("missing required field: name")
	}
	if fm.Description == "" {
		return nil, fmt.Errorf("missing required field: description")
	}

	dir := filepath.Dir(path)

	// Walk recursively from the real (resolved) directory so symlinks to
	// subdirectories are followed. On NixOS, skill files are often symlink
	// farms where every entry points into /nix/store.
	walkRoot := dir
	if realPath, err := filepath.EvalSymlinks(path); err == nil {
		walkRoot = filepath.Dir(realPath)
	}

	var files []string
	filepath.WalkDir(walkRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.EqualFold(name, "skill.md") {
			return nil
		}
		switch name {
		case ".gitignore", "README.md", "LICENSE":
			return nil
		}
		// Map back to user-facing prefix so model sees clean paths.
		userPath := strings.Replace(p, walkRoot, dir, 1)
		files = append(files, userPath)
		return nil
	})

	return &Skill{
		Name:                   fm.Name,
		Description:            fm.Description,
		WhenToUse:              fm.WhenToUse,
		Content:                strings.TrimSpace(body),
		Dir:                    dir,
		Files:                  files,
		Context:                fm.Context,
		AgentType:              fm.Agent,
		AllowedTools:           fm.AllowedTools,
		MaxSteps:               fm.MaxSteps,
		TokenBudget:            fm.TokenBudget,
		DisableModelInvocation: fm.DisableModelInvocation,
	}, nil
}

// parseFrontmatter splits SKILL.md into YAML frontmatter and markdown body.
func parseFrontmatter(content string) (*frontmatter, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		return nil, "", fmt.Errorf("frontmatter must start with ---")
	}

	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return nil, "", fmt.Errorf("unclosed frontmatter (missing closing ---)")
	}

	yamlText := rest[:end]
	body := rest[end+4:]

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(yamlText), &fm); err != nil {
		return nil, "", fmt.Errorf("invalid YAML: %w", err)
	}

	return &fm, body, nil
}
