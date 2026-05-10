package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSampleDeploySkill(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Skip(err)
	}
	root := filepath.Join(cwd, "..", "..")
	skillPath := filepath.Join(root, ".nekocode", "skills", "deploy", "skill.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		t.Skipf("sample skill not found at %s", skillPath)
	}

	sk, err := loadSkill(skillPath)
	if err != nil {
		t.Fatalf("loadSkill: %v", err)
	}

	if sk.Name != "deploy" {
		t.Errorf("name = %q", sk.Name)
	}
	if sk.Context != "fork" {
		t.Errorf("context = %q", sk.Context)
	}
	if sk.AgentType != "executor" {
		t.Errorf("agent = %q", sk.AgentType)
	}
	if len(sk.AllowedTools) != 3 {
		t.Errorf("allowed_tools = %v", sk.AllowedTools)
	}
	if sk.MaxSteps != 8 {
		t.Errorf("max_steps = %d", sk.MaxSteps)
	}
	if len(sk.Files) != 1 || !strings.HasSuffix(sk.Files[0], "healthcheck.sh") {
		t.Errorf("files = %v", sk.Files)
	}

	t.Logf("Sample skill loaded: name=%q files=%v content_len=%d", sk.Name, sk.Files, len(sk.Content))
}
