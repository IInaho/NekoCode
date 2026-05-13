// file_cache.go — FileStateCache with mtime-based invalidation for ReadTool.
// Caches file read results to avoid redundant disk I/O. Cache entries are
// invalidated when the file's mtime or size changes.

package tools

import (
	"os"
	"path/filepath"
	"sync"
)

const (
	maxCacheEntries = 100
	maxCacheBytes   = 25 << 20 // 25 MB
)

// FileState represents a cached file read result.
type FileState struct {
	Content string
	Mtime   int64 // from stat
	Size    int64
	Offset  int // 1-based start line
	Limit   int // lines read
}

// FileStateCache is an LRU cache of file read results keyed by normalized path.
// Safe for concurrent use.
type FileStateCache struct {
	mu        sync.RWMutex
	entries   map[string]*cacheEntry
	order     []string // LRU order, most recent at end
	totalSize int
}

type cacheEntry struct {
	state FileState
}

// Global cache instance shared across the session. Set from bot.New().
var GlobalFileCache *FileStateCache

func NewFileStateCache() *FileStateCache {
	return &FileStateCache{
		entries: make(map[string]*cacheEntry),
	}
}

// Get checks if a file is cached with matching mtime and read range.
// Returns (content, true) on cache hit, ("", false) on miss.
func (c *FileStateCache) Get(path string, offset, limit int) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	normPath := normalizePath(path)
	e, ok := c.entries[normPath]
	if !ok {
		return "", false
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}

	if info.ModTime().UnixNano() != e.state.Mtime || info.Size() != e.state.Size {
		// File changed — evict stale entry.
		return "", false
	}

	// Matching range check: same offset, cached limit covers requested limit.
	// Partial reads are valid hits when offset/limit match exactly.
	if offset != e.state.Offset || limit > e.state.Limit {
		return "", false
	}

	return e.state.Content, true
}

// Put stores a file read result.
func (c *FileStateCache) Put(path string, content string, offset, limit int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		return
	}

	normPath := normalizePath(path)
	state := FileState{
		Content: content,
		Mtime:   info.ModTime().UnixNano(),
		Size:    info.Size(),
		Offset:  offset,
		Limit:   limit,
	}

	// Update or insert.
	if old, ok := c.entries[normPath]; ok {
		c.totalSize -= len(old.state.Content)
		c.removeFromOrder(normPath)
	}
	c.entries[normPath] = &cacheEntry{state: state}
	c.order = append(c.order, normPath)
	c.totalSize += len(content)

	// Evict oldest entries until within limits.
	for (len(c.entries) > maxCacheEntries || c.totalSize > maxCacheBytes) && len(c.order) > 0 {
		c.evictOldest()
	}
}

// Invalidate removes a file from the cache. Called when a file is modified.
func (c *FileStateCache) Invalidate(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	normPath := normalizePath(path)
	if e, ok := c.entries[normPath]; ok {
		c.totalSize -= len(e.state.Content)
		delete(c.entries, normPath)
		c.removeFromOrder(normPath)
	}
}

// Merge combines another cache's entries into this one, keeping newer timestamps.
func (c *FileStateCache) Merge(other *FileStateCache) {
	if other == nil {
		return
	}
	other.mu.RLock()
	c.mu.Lock()
	defer c.mu.Unlock()
	defer other.mu.RUnlock()

	for path, e := range other.entries {
		if existing, ok := c.entries[path]; !ok || e.state.Mtime > existing.state.Mtime {
			if existing != nil {
				c.totalSize -= len(existing.state.Content)
				c.removeFromOrder(path)
			}
			c.entries[path] = &cacheEntry{state: e.state}
			c.order = append(c.order, path)
			c.totalSize += len(e.state.Content)
		}
	}

	for len(c.entries) > maxCacheEntries || c.totalSize > maxCacheBytes {
		c.evictOldest()
	}
}

// Clone returns a shallow copy safe for use by sub-agents.
func (c *FileStateCache) Clone() *FileStateCache {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clone := NewFileStateCache()
	for path, e := range c.entries {
		clone.entries[path] = &cacheEntry{state: e.state}
		clone.order = append(clone.order, path)
		clone.totalSize += len(e.state.Content)
	}
	return clone
}

func (c *FileStateCache) removeFromOrder(path string) {
	for i, p := range c.order {
		if p == path {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

func (c *FileStateCache) evictOldest() {
	if len(c.order) == 0 {
		return
	}
	oldest := c.order[0]
	if e, ok := c.entries[oldest]; ok {
		c.totalSize -= len(e.state.Content)
	}
	delete(c.entries, oldest)
	c.order = c.order[1:]
}

func normalizePath(p string) string {
	resolved, err := resolvePath(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(resolved)
}
