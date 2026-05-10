// Package skill provides a file-based skill system for NekoCode.
// Skills follow the Claude Code SKILL.md convention — a directory containing
// a SKILL.md file with YAML frontmatter and Markdown body.
// Skills are discovered at startup from project and user directories,
// then made available to the model via the skill tool and slash commands.
package skill

import (
	"log"
	"strings"
	"sync"
)

// Skill represents a loaded skill definition.
type Skill struct {
	Name        string   // unique identifier
	Description string   // one-line description for listing
	WhenToUse   string   // guidance for when the model should auto-invoke
	Content     string   // markdown body (without frontmatter)
	Dir         string   // absolute path to skill directory
	Files       []string // auxiliary files in the skill directory (excluding SKILL.md)

	// Execution control.
	Context                string   // "inline" (default) or "fork"
	AgentType              string   // sub-agent type for fork mode
	AllowedTools           []string // tools whitelist
	MaxSteps               int      // max reasoning steps (fork mode)
	TokenBudget            int      // token budget (fork mode)
	DisableModelInvocation bool     // forbid model from auto-invoking
}

// Registry manages all loaded skills, thread-safe.
type Registry struct {
	mu     sync.RWMutex
	skills map[string]*Skill
	loaded map[string]bool // skills currently active in context
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{
		skills: make(map[string]*Skill),
		loaded: make(map[string]bool),
	}
}

// Load discovers and loads skills from the given directories.
func (r *Registry) Load(dirs []string) error {
	paths := discoverSkills(dirs)
	loaded := 0
	for _, p := range paths {
		sk, err := loadSkill(p)
		if err != nil {
			log.Printf("skill: skipping %s: %v", p, err)
			continue
		}
		r.mu.Lock()
		if _, exists := r.skills[sk.Name]; !exists {
			r.skills[sk.Name] = sk
			loaded++
		}
		r.mu.Unlock()
	}
	if loaded > 0 {
		log.Printf("skill: loaded %d skills", loaded)
	}
	return nil
}

// Get returns a skill by name.
func (r *Registry) Get(name string) (*Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sk, ok := r.skills[name]
	return sk, ok
}

// List returns all loaded skills.
func (r *Registry) List() []*Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Skill, 0, len(r.skills))
	for _, sk := range r.skills {
		out = append(out, sk)
	}
	return out
}

// MarkLoaded marks a skill as currently active in the conversation context.
// Loaded skills are excluded from the available skills list to prevent double-loading.
func (r *Registry) MarkLoaded(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaded[name] = true
}

// IsLoaded returns whether a skill is currently active in context.
func (r *Registry) IsLoaded(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded[name]
}

// LoadedSet returns a copy of the loaded skills set for filtering.
func (r *Registry) LoadedSet() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]bool, len(r.loaded))
	for k, v := range r.loaded {
		out[k] = v
	}
	return out
}

// SkillNames returns the names of all registered skills.
func (r *Registry) SkillNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.skills))
	for _, sk := range r.skills {
		names = append(names, sk.Name)
	}
	return names
}

// NamesString returns a formatted list of skill names for error messages.
func (r *Registry) NamesString() string {
	names := r.SkillNames()
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}
