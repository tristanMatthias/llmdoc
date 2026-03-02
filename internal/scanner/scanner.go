package scanner

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/tristanmatthias/llmdoc/internal/comment"
	"github.com/tristanmatthias/llmdoc/internal/config"
)

// FileInfo describes a candidate source file found during scanning.
type FileInfo struct {
	AbsPath  string
	RelPath  string
	Syntax   comment.CommentSyntax
	Language string
	Ext      string
}

// Walk recursively scans root and returns all files that match the configured
// extensions and are not excluded by ignore patterns.
func Walk(root string, cfg *config.Config) ([]FileInfo, error) {
	extSet := make(map[string]struct{}, len(cfg.Extensions))
	for _, e := range cfg.Extensions {
		extSet[e] = struct{}{}
	}

	var files []FileInfo

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}

		// Check ignore patterns
		if matchesIgnore(rel, d.IsDir(), cfg.Ignore) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := extSet[ext]; !ok {
			return nil
		}

		syntax, ok := comment.ForExtension(ext)
		if !ok {
			return nil
		}

		files = append(files, FileInfo{
			AbsPath:  path,
			RelPath:  rel,
			Syntax:   syntax,
			Language: syntax.Language,
			Ext:      ext,
		})
		return nil
	})

	return files, err
}

// globMatch reports whether name matches pattern, silently ignoring invalid patterns.
func globMatch(pattern, name string) bool {
	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}

// matchesIgnore reports whether path matches any of the ignore patterns.
func matchesIgnore(rel string, isDir bool, patterns []string) bool {
	// Normalize to forward slashes for consistent matching
	rel = filepath.ToSlash(rel)
	if isDir {
		rel += "/"
	}

	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)

		// Directory prefix match (e.g. "vendor/" matches "vendor/foo")
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(rel, pattern) || rel == strings.TrimSuffix(pattern, "/") {
				return true
			}
			continue
		}

		base := filepath.Base(rel)

		// Glob match against the full relative path or just the filename
		if globMatch(pattern, rel) || globMatch(pattern, base) {
			return true
		}

		// Handle ** prefix patterns like "**/*.pb.go"
		if strings.HasPrefix(pattern, "**/") && globMatch(pattern[3:], base) {
			return true
		}
	}
	return false
}
