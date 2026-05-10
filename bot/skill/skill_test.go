package skill

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	skillContent := `---
name: test-skill
description: A test skill for unit testing
when_to_use: test trigger
allowed-tools:
  - bash
  - read
context: fork
agent: executor
max_steps: 4
token_budget: 8000
---

# Test Skill

This is the test skill body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "helper.txt"), []byte("helper content"), 0644); err != nil {
		t.Fatal(err)
	}

	paths := discoverSkills([]string{tmpDir})
	if len(paths) != 1 {
		t.Fatalf("expected 1 discovered skill, got %d", len(paths))
	}

	sk, err := loadSkill(paths[0])
	if err != nil {
		t.Fatalf("loadSkill: %v", err)
	}

	if sk.Name != "test-skill" {
		t.Errorf("name = %q, want %q", sk.Name, "test-skill")
	}
	if sk.Description != "A test skill for unit testing" {
		t.Errorf("description = %q", sk.Description)
	}
	if sk.WhenToUse != "test trigger" {
		t.Errorf("when_to_use = %q", sk.WhenToUse)
	}
	if sk.Context != "fork" {
		t.Errorf("context = %q", sk.Context)
	}
	if sk.AgentType != "executor" {
		t.Errorf("agent = %q", sk.AgentType)
	}
	if len(sk.AllowedTools) != 2 || sk.AllowedTools[0] != "bash" || sk.AllowedTools[1] != "read" {
		t.Errorf("allowed_tools = %v", sk.AllowedTools)
	}
	if sk.MaxSteps != 4 {
		t.Errorf("max_steps = %d", sk.MaxSteps)
	}
	if sk.TokenBudget != 8000 {
		t.Errorf("token_budget = %d", sk.TokenBudget)
	}
	if sk.Content != "# Test Skill\n\nThis is the test skill body." {
		t.Errorf("content = %q", sk.Content)
	}
	if len(sk.Files) != 1 || !strings.HasSuffix(sk.Files[0], "helper.txt") {
		t.Errorf("files = %v", sk.Files)
	}
}

func TestRegistryLoadAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test-skill
description: A test skill
---

# Body
`), 0644)

	reg := NewRegistry()
	reg.Load([]string{tmpDir})

	sk, ok := reg.Get("test-skill")
	if !ok {
		t.Fatal("skill not found in registry")
	}
	if sk.Name != "test-skill" {
		t.Errorf("name = %q", sk.Name)
	}

	list := reg.List()
	if len(list) != 1 {
		t.Errorf("List() = %d skills", len(list))
	}

	if _, ok := reg.Get("nonexistent"); ok {
		t.Error("expected false for nonexistent skill")
	}
}

func TestSkillToolExecute(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	os.MkdirAll(skillDir, 0755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test-skill
description: A test skill
---

# Test Body

Some content here.
`), 0644)

	reg := NewRegistry()
	reg.Load([]string{tmpDir})

	tool := NewSkillTool(reg)
	if tool.Name() != "skill" {
		t.Errorf("Name() = %q", tool.Name())
	}

	output, err := tool.Execute(context.Background(), map[string]interface{}{"name": "test-skill"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if !strings.Contains(output, "<skill_content") || !strings.Contains(output, "test-skill") {
		t.Errorf("output missing expected content: %s", output)
	}
	if !strings.Contains(output, "# Test Body") {
		t.Errorf("output missing body: %s", output)
	}
	if !strings.Contains(output, "Skill files") {
		t.Errorf("output missing base dir: %s", output)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{"name": "nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent skill")
	}
}

func TestBuildSkillListText(t *testing.T) {
	skills := []*Skill{
		{Name: "deploy", Description: "deploy app", WhenToUse: "when deploying"},
		{Name: "review", Description: "review code"},
	}

	text := BuildSkillListText(skills, nil, 64000)
	if text == "" {
		t.Error("expected non-empty text")
	}
	if !strings.Contains(text, "deploy") || !strings.Contains(text, "review") {
		t.Errorf("missing skill names: %s", text)
	}
	if !strings.Contains(text, "when deploying") {
		t.Errorf("missing when_to_use: %s", text)
	}
	if strings.Contains(text, "When to use: review code") {
		t.Error("should not have when_to_use for skill without one")
	}

	// Test loaded filtering: deploy is loaded, should be excluded.
	text2 := BuildSkillListText(skills, map[string]bool{"deploy": true}, 64000)
	if strings.Contains(text2, "deploy") {
		t.Error("loaded skill deploy should be excluded")
	}
	if !strings.Contains(text2, "review") {
		t.Error("unloaded skill review should still appear")
	}
	// All loaded → empty.
	text3 := BuildSkillListText(skills, map[string]bool{"deploy": true, "review": true}, 64000)
	if text3 != "" {
		t.Error("expected empty when all skills loaded")
	}

	if BuildSkillListText(nil, nil, 64000) != "" {
		t.Error("expected empty string for nil skills")
	}
	if BuildSkillListText([]*Skill{}, nil, 64000) != "" {
		t.Error("expected empty string for empty skills")
	}
}

func TestFormatForContext(t *testing.T) {
	sk := &Skill{
		Name:    "deploy",
		Content: "# Deploy\n\nStep 1: build",
		Dir:     "/home/user/.nekocode/skills/deploy",
		Files:   []string{"healthcheck.sh"},
	}

	text := FormatForContext(sk)
	if !strings.Contains(text, `<skill_content name="deploy">`) {
		t.Errorf("missing skill_content tag: %s", text)
	}
	if !strings.Contains(text, "# Skill: deploy") {
		t.Errorf("missing skill header: %s", text)
	}
	if !strings.Contains(text, "# Deploy") {
		t.Errorf("missing body: %s", text)
	}
	if !strings.Contains(text, "healthcheck.sh") {
		t.Errorf("missing file: %s", text)
	}

	sk2 := &Skill{Name: "simple", Content: "body", Dir: "/tmp/simple"}
	text2 := FormatForContext(sk2)
	if strings.Contains(text2, "<skill_files>") {
		t.Error("should not contain skill_files when no files")
	}
}
