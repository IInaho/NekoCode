package bot

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var skipDirs = map[string]bool{
	".git": true, ".claude": true, "node_modules": true,
	"__pycache__": true, ".mypy_cache": true, "target": true,
	"dist": true, "build": true, ".next": true, "vendor": true,
}

var skipExtensions = map[string]bool{
	".exe": true, ".dll": true, ".so": true, ".o": true, ".a": true,
	".zip": true, ".tar": true, ".gz": true, ".png": true, ".jpg": true,
	".svg": true, ".ico": true, ".woff": true, ".woff2": true, ".ttf": true,
	".sum": true,
}

type treeNode struct {
	name     string
	lines    int
	children map[string]*treeNode
}

type fileEntry struct {
	relPath  string
	fullPath string
}

func buildDirectoryTree(root string) string {
	n := &treeNode{children: make(map[string]*treeNode)}
	var files []fileEntry

	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if rel == "." {
			return nil
		}

		if info.IsDir() {
			if skipDirs[info.Name()] || strings.HasPrefix(info.Name(), ".") {
				return filepath.SkipDir
			}
			parts := strings.Split(filepath.ToSlash(rel), "/")
			curr := n
			for _, p := range parts {
				if curr.children[p] == nil {
					curr.children[p] = &treeNode{name: p, children: make(map[string]*treeNode)}
				}
				curr = curr.children[p]
			}
			return nil
		}

		if skipExtensions[filepath.Ext(info.Name())] {
			return nil
		}
		files = append(files, fileEntry{relPath: rel, fullPath: path})
		return nil
	})

	// Count lines concurrently.
	countLinesConcurrent(files, n, root)

	var b strings.Builder
	b.WriteString("\n## 工作目录结构\n\n```\n")
	renderTree(&b, n, "")
	b.WriteString("```")
	return b.String()
}

func countLinesConcurrent(files []fileEntry, root *treeNode, rootPath string) {
	const numWorkers = 8
	type result struct {
		relPath string
		lines   int
	}

	jobs := make(chan fileEntry, len(files))
	results := make(chan result, len(files))

	var wg sync.WaitGroup
	for i := 0; i < min(numWorkers, len(files)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				results <- result{f.relPath, countLines(f.fullPath)}
			}
		}()
	}

	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		parts := strings.Split(filepath.ToSlash(r.relPath), "/")
		curr := root
		for i := 0; i < len(parts)-1; i++ {
			curr = curr.children[parts[i]]
		}
		fname := parts[len(parts)-1]
		if child := curr.children[fname]; child != nil {
			child.lines = r.lines
		}
	}
}

func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
	}
	return n
}

func renderTree(b *strings.Builder, n *treeNode, prefix string) {
	type entry struct {
		name  string
		node  *treeNode
		isDir bool
	}
	var entries []entry
	for name, child := range n.children {
		isDir := child.lines == 0 && len(child.children) > 0
		entries = append(entries, entry{name, child, isDir})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].isDir != entries[j].isDir {
			return entries[i].isDir
		}
		return entries[i].name < entries[j].name
	})

	for i, e := range entries {
		last := i == len(entries)-1
		conn := "├── "
		childPre := "│   "
		if last {
			conn = "└── "
			childPre = "    "
		}

		if e.isDir {
			b.WriteString(prefix + conn + e.name + "/\n")
			renderTree(b, e.node, prefix+childPre)
		} else {
			b.WriteString(prefix + conn)
			b.WriteString(e.name)
			if e.node.lines > 0 {
				tag := ""
				if e.node.lines >= 500 {
					tag = " LARGE"
				}
				b.WriteString(" (" + strconv.Itoa(e.node.lines) + "L" + tag + ")")
			}
			b.WriteString("\n")
		}
	}
}
