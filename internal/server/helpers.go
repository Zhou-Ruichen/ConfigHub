package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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
	chtmpl "github.com/ruichen/config-hub/internal/template"
)

type profileSummary struct {
	ID                  string   `json:"id"`
	Owner               string   `json:"owner"`
	Role                string   `json:"role"`
	Domains             []string `json:"domains"`
	LatestBundleVersion string   `json:"latestBundleVersion,omitempty"`
	BundleCount         int      `json:"bundleCount"`
	LastRenderedAt      string   `json:"lastRenderedAt,omitempty"`
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Printf("request panic method=%s path=%s", r.Method, r.URL.Path)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := randomID()
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.logger.Printf("request id=%s method=%s path=%s status=%d duration=%s", reqID, r.Method, r.URL.Path, rec.status, time.Since(start).Truncate(time.Millisecond))
	})
}

func (s *Server) authorize(r *http.Request, required string, hideUnauthorized bool) (*secret.Token, bool) {
	if s.loopbackOnly {
		return &secret.Token{ID: "loopback", Scope: "admin"}, true
	}
	var tok *secret.Token
	if bearer := bearerToken(r.Header.Get("Authorization")); bearer != "" {
		found, err := secret.Lookup(s.rootDir, bearer)
		if err == nil {
			tok = found
		}
	}
	if tok == nil {
		if cookieTok, ok := s.tokenFromCookie(r); ok {
			tok = cookieTok
		}
	}
	if tok == nil {
		return nil, false
	}
	if required == "any" || tok.Scope == "admin" || tok.Scope == required {
		return tok, true
	}
	_ = hideUnauthorized
	return nil, false
}

func (s *Server) writeAuthFailure(w http.ResponseWriter, r *http.Request) {
	bearer := bearerToken(r.Header.Get("Authorization"))
	if bearer != "" {
		if _, err := secret.Lookup(s.rootDir, bearer); err == nil {
			writeError(w, http.StatusForbidden, "forbidden", "token scope does not permit this resource")
			return
		}
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}
	if cookie, err := r.Cookie("confighub_session"); err == nil && cookie.Value != "" {
		if _, ok := s.tokenFromCookie(r); ok {
			writeError(w, http.StatusForbidden, "forbidden", "token scope does not permit this resource")
			return
		}
	}
	writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
}

func bearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, tokenID string) {
	value := tokenID + ":" + s.signSession(tokenID)
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{Name: "confighub_session", Value: value, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: secure})
}

func (s *Server) tokenFromCookie(r *http.Request) (*secret.Token, bool) {
	cookie, err := r.Cookie("confighub_session")
	if err != nil {
		return nil, false
	}
	parts := strings.SplitN(cookie.Value, ":", 2)
	if len(parts) != 2 {
		return nil, false
	}
	if !hmac.Equal([]byte(parts[1]), []byte(s.signSession(parts[0]))) {
		return nil, false
	}
	tokens, err := secret.List(s.rootDir)
	if err != nil {
		return nil, false
	}
	for i := range tokens {
		if tokens[i].ID == parts[0] {
			return &tokens[i], true
		}
	}
	return nil, false
}

func (s *Server) signSession(tokenID string) string {
	mac := hmac.New(sha256.New, s.sessionKey)
	_, _ = mac.Write([]byte(tokenID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Server) listProfiles() ([]profileSummary, error) {
	dir := filepath.Join(s.rootDir, "profiles")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []profileSummary{}, nil
		}
		return nil, fmt.Errorf("read profiles: %w", err)
	}
	out := make([]profileSummary, 0)
	for _, entry := range entries {
		if entry.IsDir() || !(strings.HasSuffix(entry.Name(), ".yaml") || strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		prof, err := profile.Load(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		domains := make([]string, 0, len(prof.Domains))
		for domain, enabled := range prof.Domains {
			if enabled {
				domains = append(domains, domain)
			}
		}
		sort.Strings(domains)
		versions, latest, last := s.bundleStats(prof.ID)
		out = append(out, profileSummary{ID: prof.ID, Owner: prof.Owner, Role: prof.Role, Domains: domains, BundleCount: versions, LatestBundleVersion: latest, LastRenderedAt: last})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Server) bundleStats(profileID string) (int, string, string) {
	dir := filepath.Join(s.rootDir, "bundles", profileID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, "", ""
	}
	versions := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	sort.Strings(versions)
	if len(versions) == 0 {
		return 0, "", ""
	}
	latest := versions[len(versions)-1]
	manifest, err := bundle.LoadManifest(filepath.Join(dir, latest, "manifest.json"))
	if err != nil {
		return len(versions), latest, ""
	}
	return len(versions), latest, manifest.CreatedAt
}

func isRemoteDelivery(delivery string) bool {
	return delivery == "remote" || delivery == "both"
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"code": code, "message": msg})
}

func checksumETag(data []byte) string {
	sum := sha256.Sum256(data)
	return "\"" + hex.EncodeToString(sum[:]) + "\""
}

func matchETag(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(part) == etag {
			return true
		}
	}
	return false
}

func randomID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(buf)
}

func loadOrCreateSessionKey(root string) ([]byte, error) {
	stateDir := filepath.Join(root, "state")
	path := filepath.Join(stateDir, "session.key")
	data, err := os.ReadFile(path)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("decode session key: %w", err)
		}
		return decoded, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read session key: %w", err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate session key: %w", err)
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	_ = os.Chmod(stateDir, 0o700)
	encoded := []byte(base64.RawURLEncoding.EncodeToString(key) + "\n")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return nil, fmt.Errorf("write session key: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return key, nil
}

func profileTemplateEntries(root string, prof *profile.Profile) []string {
	if prof == nil {
		return nil
	}
	ids := append([]string(nil), prof.AllowedTemplates...)
	sort.Strings(ids)
	return ids
}

func loadTemplateDef(root, id string) (*chtmpl.Template, error) {
	return chtmpl.Load(filepath.Join(root, "templates", filepath.FromSlash(id)+".yaml"))
}
