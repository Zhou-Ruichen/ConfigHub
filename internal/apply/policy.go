package apply

import (
	"fmt"
	"path/filepath"
	"strings"
)

var homeDotfiles = map[string]bool{
	".gitconfig":          true,
	".gitconfig.local":    true,
	".zshrc":              true,
	".zshrc.local":        true,
	".zshenv":             true,
	".zshenv.local":       true,
	".zprofile":           true,
	".zprofile.local":     true,
	".bashrc":             true,
	".bashrc.local":       true,
	".bash_profile":       true,
	".bash_profile.local": true,
	".profile":            true,
	".tmux.conf":          true,
	".vimrc":              true,
}

// ValidateTargetPath expands ~, cleans the result, rejects traversal, and
// enforces ConfigHub's allowlisted roots plus forbidden sensitive patterns.
func ValidateTargetPath(homeDir, targetPath string) (string, error) {
	if homeDir == "" {
		return "", fmt.Errorf("%w: home directory is required", ErrPathPolicy)
	}
	absHome, err := filepath.Abs(homeDir)
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	absHome = filepath.Clean(absHome)
	if targetPath == "" {
		return "", fmt.Errorf("%w: empty target path", ErrPathPolicy)
	}

	expanded := targetPath
	if targetPath == "~" {
		expanded = absHome
	} else if strings.HasPrefix(targetPath, "~/") {
		expanded = filepath.Join(absHome, targetPath[2:])
	}
	cleaned := filepath.Clean(expanded)
	if !filepath.IsAbs(cleaned) {
		cleaned = filepath.Clean(filepath.Join(absHome, cleaned))
	}

	if hasDotDotSegment(targetPath) || hasDotDotSegment(cleaned) {
		return "", fmt.Errorf("%w: path %q contains ..", ErrPathPolicy, targetPath)
	}
	if cleaned != absHome && !strings.HasPrefix(cleaned, absHome+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: path %q escapes home", ErrPathPolicy, targetPath)
	}
	rel, err := filepath.Rel(absHome, cleaned)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%w: path %q escapes home", ErrPathPolicy, targetPath)
	}
	relSlash := filepath.ToSlash(rel)
	base := filepath.Base(cleaned)

	if strings.HasPrefix(relSlash, ".ssh/") || relSlash == ".ssh" || strings.HasPrefix(relSlash, ".gnupg/") || relSlash == ".gnupg" {
		return "", fmt.Errorf("%w: forbidden sensitive root %q", ErrPathPolicy, targetPath)
	}
	if base == ".cache" || strings.Contains(base, "history") || strings.HasSuffix(base, ".sqlite") || strings.HasSuffix(base, ".db") || strings.HasSuffix(base, ".kdbx") {
		return "", fmt.Errorf("%w: forbidden target pattern %q", ErrPathPolicy, targetPath)
	}

	allowed := false
	if !strings.Contains(relSlash, "/") && homeDotfiles[relSlash] {
		allowed = true
	}
	for _, prefix := range []string{".confighub/", ".config/", ".codex/", ".claude/", ".local/share/"} {
		if strings.HasPrefix(relSlash, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("%w: target %q is not in an allowlisted root", ErrPathPolicy, targetPath)
	}
	return cleaned, nil
}

func hasDotDotSegment(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}
