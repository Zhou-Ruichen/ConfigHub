package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/profile"
	"github.com/ruichen/config-hub/internal/secret"
	"github.com/ruichen/config-hub/internal/web"
)

type templateView struct {
	ID           string
	Domain       string
	Target       string
	Delivery     string
	Secrets      string
	SecretsClass string
}

func (s *Server) renderStatusPage(w http.ResponseWriter, r *http.Request) {
	profiles, _ := s.listProfiles()
	tokens, _ := secret.List(s.rootDir)
	bundleCount := 0
	for _, prof := range profiles {
		bundleCount += prof.BundleCount
	}
	s.renderHTML(w, "status", "Status", map[string]any{
		"Version":      s.version,
		"ProfileCount": len(profiles),
		"BundleCount":  bundleCount,
		"TokenCount":   len(tokens),
		"Uptime":       time.Since(s.start).Truncate(time.Second).String(),
	})
}

func (s *Server) renderProfilesPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorize(r, "any", false); !ok {
		s.renderHTMLStatus(w, http.StatusUnauthorized, "warnings", "Unauthorized", map[string]any{"Warnings": []string{"Missing or invalid token."}})
		return
	}
	profiles, _ := s.listProfiles()
	s.renderHTML(w, "profiles", "Profiles", map[string]any{"Profiles": profiles})
}

func (s *Server) renderProfileDetailPage(w http.ResponseWriter, r *http.Request, profileID string) {
	if _, ok := s.authorize(r, "pull:"+profileID, false); !ok {
		s.renderHTMLStatus(w, http.StatusForbidden, "warnings", "Forbidden", map[string]any{"Warnings": []string{"Token scope does not permit this profile."}})
		return
	}
	path := filepath.Join(s.rootDir, "profiles", profileID+".yaml")
	prof, err := profile.Load(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	data, _ := os.ReadFile(path)
	latest, _ := bundle.LatestBundleVersion(filepath.Join(s.rootDir, "bundles"), profileID)
	views := make([]templateView, 0, len(prof.AllowedTemplates))
	for _, id := range profileTemplateEntries(s.rootDir, prof) {
		def, err := loadTemplateDef(s.rootDir, id)
		if err != nil {
			views = append(views, templateView{ID: id, Delivery: "missing"})
			continue
		}
		secretsClass := ""
		if def.Safety.Secrets == "allowed" {
			secretsClass = "danger"
		}
		views = append(views, templateView{ID: def.ID, Domain: def.Domain, Target: def.Target, Delivery: deliveryLabel(def.Delivery.Sync, def.Delivery.Remote), Secrets: def.Safety.Secrets, SecretsClass: secretsClass})
	}
	s.renderHTML(w, "profile_detail", "Profile "+profileID, map[string]any{
		"Profile":      prof,
		"YAML":         string(data),
		"Templates":    views,
		"LatestBundle": latest,
		"Bootstrap":    bootstrapCommand(profileID),
	})
}

func (s *Server) renderBundlePage(w http.ResponseWriter, r *http.Request, profileID, version string) {
	if _, ok := s.authorize(r, "pull:"+profileID, false); !ok {
		s.renderHTMLStatus(w, http.StatusForbidden, "warnings", "Forbidden", map[string]any{"Warnings": []string{"Token scope does not permit this profile."}})
		return
	}
	manifest, err := bundle.LoadManifest(filepath.Join(s.rootDir, "bundles", profileID, version, "manifest.json"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	s.renderHTML(w, "bundle", "Bundle "+version, map[string]any{"Manifest": manifest, "Bootstrap": bootstrapCommand(profileID)})
}

func (s *Server) renderBootstrapPage(w http.ResponseWriter, r *http.Request, profileID string) {
	if _, ok := s.authorize(r, "pull:"+profileID, false); !ok {
		s.renderHTMLStatus(w, http.StatusForbidden, "warnings", "Forbidden", map[string]any{"Warnings": []string{"Token scope does not permit this profile."}})
		return
	}
	s.renderHTML(w, "bootstrap", "Bootstrap "+profileID, map[string]any{"ProfileID": profileID, "Command": bootstrapCommand(profileID)})
}

func (s *Server) renderWarningsPage(w http.ResponseWriter, r *http.Request) {
	warnings := make([]string, 0)
	profiles, _ := s.listProfiles()
	if !s.loopbackOnly {
		tokens, _ := secret.List(s.rootDir)
		if len(tokens) == 0 {
			warnings = append(warnings, "No tokens configured on a non-loopback bind.")
		}
	}
	for _, prof := range profiles {
		if prof.BundleCount == 0 {
			warnings = append(warnings, fmt.Sprintf("Profile %s has no rendered bundle.", prof.ID))
		}
		p, err := profile.Load(filepath.Join(s.rootDir, "profiles", prof.ID+".yaml"))
		if err != nil {
			continue
		}
		for _, id := range p.AllowedTemplates {
			def, err := loadTemplateDef(s.rootDir, id)
			if err == nil && def.Safety.Secrets == "allowed" {
				warnings = append(warnings, fmt.Sprintf("Template %s declares secrets: allowed.", id))
			}
		}
	}
	sort.Strings(warnings)
	s.renderHTML(w, "warnings", "Warnings", map[string]any{"Warnings": warnings})
}

func (s *Server) renderHTML(w http.ResponseWriter, page, title string, data any) {
	s.renderHTMLStatus(w, http.StatusOK, page, title, data)
}

func (s *Server) renderHTMLStatus(w http.ResponseWriter, status int, page, title string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := web.Render(w, page, web.PageData{Title: title, Data: data}); err != nil {
		s.logger.Printf("render html page=%s error=%v", page, err)
	}
}

func deliveryLabel(sync, remote bool) string {
	switch {
	case sync && remote:
		return "both"
	case remote:
		return "remote"
	case sync:
		return "sync"
	default:
		return "none"
	}
}

func bootstrapCommand(profileID string) string {
	return fmt.Sprintf("confighub pull --from http://127.0.0.1:8787 --profile %s --dry-run", shellSafe(profileID))
}

func shellSafe(s string) string {
	return strings.ReplaceAll(s, "'", "")
}
