package skill

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// discoverSkills scans the given directories for skill definition files.
// Recognizes both skill.md and SKILL.md.
// Returns absolute paths sorted so project dirs (scanned first) take priority.
func discoverSkills(dirs []string) []string {
	var paths []string
	seen := make(map[string]bool)

	for _, dir := range dirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absDir); os.IsNotExist(err) {
			continue
		}
		filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			name := strings.ToLower(info.Name())
			if name == "skill.md" {
				// Use the absolute path as-is; do NOT resolve symlinks.
				// On NixOS, resolving symlinks leaks /nix/store paths
				// into the skill Dir, confusing the model.
				absPath, _ := filepath.Abs(path)
				if !seen[absPath] {
					seen[absPath] = true
					paths = append(paths, absPath)
				}
				return filepath.SkipDir
			}
			return nil
		})
	}

	sort.Strings(paths)
	return paths
}

// DefaultDirs returns the default skill directories (project + user).
func DefaultDirs() []string {
	var dirs []string

	// Project-level skill directory.
	if cwd, err := os.Getwd(); err == nil {
		dirs = append(dirs, filepath.Join(cwd, ".nekocode", "skills"))
	}

	// User-level skill directory.
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".nekocode", "skills"))
	}

	return dirs
}
