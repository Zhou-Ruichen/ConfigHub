package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/profile"
	chtmpl "github.com/ruichen/config-hub/internal/template"
)

// Options configures an in-memory render. The caller is responsible for lock
// acquisition and deciding whether to persist the returned bundle.
type Options struct {
	ProfilePath string
	RootDir     string
	DryRun      bool
	JSON        bool
}

// Result contains all data needed to write or inspect a rendered bundle.
type Result struct {
	Profile   *profile.Profile
	Files     map[string][]byte
	Manifest  *bundle.Manifest
	Checksums map[string]string
}

// Context is exposed to text/template source files.
type Context struct {
	Profile *profile.Profile
	Vars    map[string]any
	Env     map[string]string
	Secrets map[string]any
}

// Render loads a profile, renders its allowlisted templates, computes checksums,
// and builds a manifest. It does not write persistent output.
func Render(ctx context.Context, opts Options) (*Result, error) {
	rootDir := opts.RootDir
	if rootDir == "" {
		rootDir = "."
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}

	prof, err := profile.Load(opts.ProfilePath)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	version, err := nextBundleVersion(filepath.Join(absRoot, "bundles"), prof.ID, now)
	if err != nil {
		return nil, err
	}

	files := make(map[string][]byte)
	checksums := make(map[string]string)
	entries := make([]bundle.FileEntry, 0, len(prof.AllowedTemplates))
	domainSet := make(map[string]bool)

	for _, templateID := range prof.AllowedTemplates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		defPath := filepath.Join(absRoot, "templates", filepath.FromSlash(templateID)+".yaml")
		def, err := chtmpl.Load(defPath)
		if err != nil {
			return nil, err
		}
		if def.ID != templateID {
			return nil, fmt.Errorf("validate template %q: id mismatch %q", templateID, def.ID)
		}

		rendered, err := executeTemplate(absRoot, prof, def)
		if err != nil {
			return nil, fmt.Errorf("render template %q: %w", def.ID, err)
		}
		checksum := sha256Checksum(rendered)
		bundlePath := bundlePathFor(def)
		files[bundlePath] = rendered
		checksums[bundlePath] = checksum
		entries = append(entries, bundle.FileEntry{
			TemplateID: def.ID,
			Domain:     def.Domain,
			BundlePath: bundlePath,
			TargetPath: def.Target,
			Mode:       def.Mode,
			Checksum:   checksum,
			Delivery:   deliveryString(def.Delivery),
			Safety: bundle.Safety{
				Backup:          def.Safety.Backup,
				Diff:            def.Safety.Diff,
				Symlink:         def.Safety.Symlink,
				Secrets:         def.Safety.Secrets,
				Merge:           def.Safety.Merge,
				IncludeStrategy: def.Safety.IncludeStrategy,
			},
		})
		domainSet[def.Domain] = true
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TemplateID < entries[j].TemplateID
	})

	domains := make([]string, 0, len(domainSet))
	for domain := range domainSet {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	removed, err := removedFiles(filepath.Join(absRoot, "bundles"), prof.ID, entries)
	if err != nil {
		return nil, err
	}

	manifest := &bundle.Manifest{
		SchemaVersion:  bundle.SupportedSchemaVersion,
		BundleVersion:  version,
		ProfileID:      prof.ID,
		CreatedAt:      now.Format(time.RFC3339),
		SourceRevision: sourceRevision(absRoot),
		Domains:        domains,
		Files:          entries,
		RemovedFiles:   removed,
		ChangeSummary:  "",
		Signature:      nil,
	}
	if err := bundle.Validate(manifest); err != nil {
		return nil, err
	}

	return &Result{Profile: prof, Files: files, Manifest: manifest, Checksums: checksums}, nil
}

func executeTemplate(root string, prof *profile.Profile, def *chtmpl.Template) ([]byte, error) {
	sourcePath := filepath.Join(root, "templates", filepath.FromSlash(def.Source))
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read source %q: %w", sourcePath, err)
	}

	envAllowlist := make(map[string]bool, len(def.EnvAllowlist))
	env := make(map[string]string, len(def.EnvAllowlist))
	for _, name := range def.EnvAllowlist {
		envAllowlist[name] = true
		env[name] = os.Getenv(name)
	}
	vars := prof.Vars
	if vars == nil {
		vars = map[string]any{}
	}

	funcs := texttemplate.FuncMap{
		"secret": func(name string) (string, error) {
			return "", fmt.Errorf("secret resolution is not implemented before Slice 6")
		},
		"env": func(name string) (string, error) {
			if !envAllowlist[name] {
				return "", fmt.Errorf("env %q is not in this template's envAllowlist", name)
			}
			return os.Getenv(name), nil
		},
		"var": func(name string) (any, error) {
			value, ok := vars[name]
			if !ok {
				return nil, fmt.Errorf("var %q is not defined", name)
			}
			return value, nil
		},
	}

	parsed, err := texttemplate.New(filepath.Base(sourcePath)).Funcs(funcs).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse source %q: %w", sourcePath, err)
	}
	renderCtx := Context{Profile: prof, Vars: vars, Env: env, Secrets: map[string]any{}}
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, renderCtx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func bundlePathFor(def *chtmpl.Template) string {
	if def.ID == "dotfiles/git-include" {
		return "files/dotfiles/git/include-line.txt"
	}
	base := strings.TrimSuffix(path.Base(def.Source), ".tmpl")
	dir := path.Dir(filepath.ToSlash(def.Source))
	return path.Join("files", dir, base)
}

func deliveryString(delivery chtmpl.Delivery) string {
	switch {
	case delivery.Sync && delivery.Remote:
		return "both"
	case delivery.Remote:
		return "remote"
	default:
		return "sync"
	}
}

func sha256Checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func removedFiles(rootBundlesDir, profileID string, current []bundle.FileEntry) ([]bundle.RemovedFileEntry, error) {
	latest, err := bundle.LatestBundleVersion(rootBundlesDir, profileID)
	if err != nil {
		return nil, err
	}
	if latest == "" {
		return []bundle.RemovedFileEntry{}, nil
	}
	previous, err := bundle.LoadManifest(filepath.Join(rootBundlesDir, profileID, latest, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("load previous manifest: %w", err)
	}
	currentByTarget := make(map[string]bundle.FileEntry, len(current))
	for _, entry := range current {
		currentByTarget[entry.TargetPath] = entry
	}
	removed := make([]bundle.RemovedFileEntry, 0)
	for _, entry := range previous.Files {
		if _, ok := currentByTarget[entry.TargetPath]; !ok {
			removed = append(removed, bundle.RemovedFileEntry{
				TemplateID:       entry.TemplateID,
				TargetPath:       entry.TargetPath,
				Reason:           "removed",
				PreviousChecksum: entry.Checksum,
			})
		}
	}
	sort.Slice(removed, func(i, j int) bool {
		return removed[i].TargetPath < removed[j].TargetPath
	})
	return removed, nil
}

func nextBundleVersion(rootBundlesDir, profileID string, now time.Time) (string, error) {
	prefix := now.Format("2006-01-02T15-04-05Z")
	for seq := 1; seq <= 999; seq++ {
		version := fmt.Sprintf("%s-%03d", prefix, seq)
		if _, err := os.Stat(filepath.Join(rootBundlesDir, profileID, version)); err != nil {
			if os.IsNotExist(err) {
				return version, nil
			}
			return "", fmt.Errorf("check bundle version %q: %w", version, err)
		}
	}
	return "", fmt.Errorf("allocate bundle version: exhausted sequence for %s", prefix)
}

func sourceRevision(root string) string {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "none"
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "none"
	}
	return "git:" + sha
}
