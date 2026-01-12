package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// HashCache provides ModTime-based caching for file hashes
type HashCache struct {
	mu      sync.RWMutex
	entries map[string]hashEntry
}

type hashEntry struct {
	modTime time.Time
	size    int64
	hash    string
}

// Global hash cache instance
var globalHashCache = &HashCache{
	entries: make(map[string]hashEntry),
}

// GetHashCache returns the global hash cache
func GetHashCache() *HashCache {
	return globalHashCache
}

// GetOrCompute returns cached hash if file hasn't changed, otherwise computes new hash
func (c *HashCache) GetOrCompute(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	// Check cache
	c.mu.RLock()
	entry, ok := c.entries[path]
	c.mu.RUnlock()

	// Cache hit: file hasn't changed
	if ok && entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		return entry.hash, nil
	}

	// Cache miss: compute hash
	var hash string
	if info.IsDir() {
		hash, err = computeDirHashInternal(path)
	} else {
		hash, err = computeFileHashInternal(path)
	}
	if err != nil {
		return "", err
	}

	// Store in cache
	c.mu.Lock()
	c.entries[path] = hashEntry{
		modTime: info.ModTime(),
		size:    info.Size(),
		hash:    hash,
	}
	c.mu.Unlock()

	return hash, nil
}

// Clear clears the hash cache
func (c *HashCache) Clear() {
	c.mu.Lock()
	c.entries = make(map[string]hashEntry)
	c.mu.Unlock()
}

// CacheSize returns the number of cached entries
func (c *HashCache) CacheSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// ComputeFileHash computes SHA256 hash of a file (uses cache)
func ComputeFileHash(path string) (string, error) {
	return globalHashCache.GetOrCompute(path)
}

// computeFileHashInternal computes SHA256 hash without caching
func computeFileHashInternal(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// ComputeDirHash computes a combined hash of all files in a directory
func ComputeDirHash(dirPath string) (string, error) {
	return globalHashCache.GetOrCompute(dirPath)
}

// ComputeDirHashNoCache computes directory hash without caching (useful for tests)
func ComputeDirHashNoCache(dirPath string) (string, error) {
	return computeDirHashInternal(dirPath)
}

// ComputeFileHashNoCache computes file hash without caching (useful for tests)
func ComputeFileHashNoCache(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		return computeDirHashInternal(path)
	}
	return computeFileHashInternal(path)
}

// InvalidatePath removes a path from the cache
func (c *HashCache) InvalidatePath(path string) {
	c.mu.Lock()
	delete(c.entries, path)
	c.mu.Unlock()
}

// computeDirHashInternal computes directory hash without caching
func computeDirHashInternal(dirPath string) (string, error) {
	hasher := sha256.New()

	var filePaths []string
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !d.IsDir() && !shouldSkipFile(d.Name()) {
			relPath, _ := filepath.Rel(dirPath, path)
			filePaths = append(filePaths, relPath)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// Sort for consistent ordering
	sort.Strings(filePaths)

	for _, relPath := range filePaths {
		fullPath := filepath.Join(dirPath, relPath)

		// Hash the relative path
		hasher.Write([]byte(relPath))

		// Hash the file content
		file, err := os.Open(fullPath)
		if err != nil {
			continue
		}
		io.Copy(hasher, file)
		file.Close()
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// QuickHash returns first 8 chars of hash for display
func QuickHash(hash string) string {
	if len(hash) >= 8 {
		return hash[:8]
	}
	return hash
}
