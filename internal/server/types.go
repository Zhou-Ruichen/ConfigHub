package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ruichen/config-hub/internal/bundle"
	"github.com/ruichen/config-hub/internal/profile"
	"github.com/ruichen/config-hub/internal/secret"
	"github.com/ruichen/config-hub/internal/web"
)

// Config describes server construction options.
type Config struct {
	RootDir      string
	LoopbackOnly bool
	AllowNoToken bool
	Version      string
	Logger       *log.Logger
	SessionKey   []byte
	StartTime    time.Time
}

// Server is the ConfigHub HTTP API and server-rendered Web UI.
type Server struct {
	rootDir      string
	loopbackOnly bool
	version      string
	start        time.Time
	logger       *log.Logger
	sessionKey   []byte
	handler      http.Handler
}

var ErrNoTokenConfigured = errors.New("non-loopback bind requires at least one token; create one with `confighub token create --label <l> --scope <s>`")

// New validates configuration and returns a server. loopbackOnly means the
// listener is local development only and may run without configured tokens.
func New(rootDir string, loopbackOnly bool, allowNoToken bool) (*Server, error) {
	return NewWithConfig(Config{RootDir: rootDir, LoopbackOnly: loopbackOnly, AllowNoToken: allowNoToken, Version: "0.1.0-dev"})
}

// NewWithConfig is New plus test/CLI injectable settings.
func NewWithConfig(cfg Config) (*Server, error) {
	root := cfg.RootDir
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root dir: %w", err)
	}
	tokens, err := secret.List(absRoot)
	if err != nil {
		return nil, err
	}
	if !cfg.LoopbackOnly && len(tokens) == 0 && !cfg.AllowNoToken {
		return nil, ErrNoTokenConfigured
	}
	logger := cfg.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	version := cfg.Version
	if version == "" {
		version = "0.1.0-dev"
	}
	start := cfg.StartTime
	if start.IsZero() {
		start = time.Now()
	}
	key := cfg.SessionKey
	if len(key) == 0 {
		key, err = loadOrCreateSessionKey(absRoot)
		if err != nil {
			return nil, err
		}
	}
	s := &Server{rootDir: absRoot, loopbackOnly: cfg.LoopbackOnly, version: version, start: start, logger: logger, sessionKey: key}
	s.handler = s.recoverMiddleware(s.loggingMiddleware(s.routes()))
	return s, nil
}

func (s *Server) Handler() http.Handler { return s.handler }
func (s *Server) RootDir() string       { return s.rootDir }

// IsLoopbackBind reports whether addr targets localhost/loopback only.
func IsLoopbackBind(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", s.handleAPIStatus)
	mux.HandleFunc("GET /api/v1/profiles", s.handleAPIProfiles)
	mux.HandleFunc("GET /api/v1/profiles/", s.handleAPIProfileSubroutes)
	mux.HandleFunc("GET /", s.handleWeb)
	return mux
}

func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	profiles, _ := s.listProfiles()
	tokens, _ := secret.List(s.rootDir)
	writeJSON(w, http.StatusOK, map[string]any{
		"version":  s.version,
		"profiles": len(profiles),
		"tokens":   len(tokens),
		"uptime":   time.Since(s.start).Truncate(time.Second).String(),
	})
}

func (s *Server) handleAPIProfiles(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorize(r, "any", false); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}
	profiles, err := s.listProfiles()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to list profiles")
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleAPIProfileSubroutes(w http.ResponseWriter, r *http.Request) {
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/profiles/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	profileID := parts[0]
	if len(parts) == 1 {
		s.handleAPIProfile(w, r, profileID)
		return
	}
	if parts[1] != "bundle" && parts[1] != "templates" {
		writeError(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	if parts[1] == "bundle" {
		s.handleAPIBundleRoutes(w, r, profileID, parts[2:])
		return
	}
	templateID := strings.Join(parts[2:], "/")
	s.handleAPITemplate(w, r, profileID, templateID)
}

func (s *Server) handleAPIProfile(w http.ResponseWriter, r *http.Request, profileID string) {
	if _, ok := s.authorize(r, "pull:"+profileID, false); !ok {
		s.writeAuthFailure(w, r)
		return
	}
	prof, err := profile.Load(filepath.Join(s.rootDir, "profiles", profileID+".yaml"))
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "profile not found")
		return
	}
	latest, _ := bundle.LatestBundleVersion(filepath.Join(s.rootDir, "bundles"), profileID)
	writeJSON(w, http.StatusOK, map[string]any{"profile": prof, "latestBundleVersion": latest})
}

func (s *Server) handleAPIBundleRoutes(w http.ResponseWriter, r *http.Request, profileID string, rest []string) {
	if len(rest) == 1 && rest[0] == "signature" {
		writeError(w, http.StatusNotFound, "not_found", "signature not available")
		return
	}
	if _, ok := s.authorize(r, "pull:"+profileID, false); !ok {
		s.writeAuthFailure(w, r)
		return
	}
	latest, err := bundle.LatestBundleVersion(filepath.Join(s.rootDir, "bundles"), profileID)
	if err != nil || latest == "" {
		writeError(w, http.StatusNotFound, "not_found", "bundle not found")
		return
	}
	bundleDir := filepath.Join(s.rootDir, "bundles", profileID, latest)
	if len(rest) == 0 {
		s.serveBundleArchive(w, r, bundleDir)
		return
	}
	if len(rest) == 1 && rest[0] == "manifest" {
		http.ServeFile(w, r, filepath.Join(bundleDir, "manifest.json"))
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "not found")
}

func (s *Server) handleAPITemplate(w http.ResponseWriter, r *http.Request, profileID, templateID string) {
	if templateID == "" {
		writeError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	latest, err := bundle.LatestBundleVersion(filepath.Join(s.rootDir, "bundles"), profileID)
	if err != nil || latest == "" {
		writeError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	manifest, err := bundle.LoadManifest(filepath.Join(s.rootDir, "bundles", profileID, latest, "manifest.json"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to load manifest")
		return
	}
	var entry *bundle.FileEntry
	for i := range manifest.Files {
		if manifest.Files[i].TemplateID == templateID {
			entry = &manifest.Files[i]
			break
		}
	}
	if entry == nil {
		writeError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	def, err := loadTemplateDef(s.rootDir, templateID)
	if err != nil || !def.Delivery.Remote || !isRemoteDelivery(entry.Delivery) {
		writeError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	secretBearing := entry.Safety.Secrets == "allowed"
	if secretBearing {
		if _, ok := s.authorize(r, "read:"+templateID, true); !ok {
			writeError(w, http.StatusNotFound, "not_found", "template not found")
			return
		}
	} else if _, ok := s.authorize(r, "any", false); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing or invalid token")
		return
	}
	path := filepath.Join(s.rootDir, "bundles", profileID, latest, filepath.FromSlash(entry.BundlePath))
	data, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	etag := checksumETag(data)
	if matchETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if t, err := time.Parse(time.RFC3339, manifest.CreatedAt); err == nil {
		w.Header().Set("Last-Modified", t.UTC().Format(http.TimeFormat))
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-ConfigHub-Profile", profileID)
	w.Header().Set("X-ConfigHub-Bundle", latest)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) serveBundleArchive(w http.ResponseWriter, r *http.Request, bundleDir string) {
	var buf bytes.Buffer
	if err := bundle.TarGz(bundleDir, &buf); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to build archive")
		return
	}
	etag := checksumETag(buf.Bytes())
	if matchETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, bytes.NewReader(buf.Bytes()))
}

func (s *Server) handleWeb(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("token") != "" {
		tok, err := secret.Lookup(s.rootDir, r.URL.Query().Get("token"))
		if err == nil {
			s.setSessionCookie(w, r, tok.ID)
		}
	}
	path := strings.Trim(r.URL.Path, "/")
	if path == "" {
		s.renderStatusPage(w, r)
		return
	}
	parts := strings.Split(path, "/")
	switch {
	case path == "profiles":
		s.renderProfilesPage(w, r)
	case path == "warnings":
		s.renderWarningsPage(w, r)
	case len(parts) == 2 && parts[0] == "profiles":
		s.renderProfileDetailPage(w, r, parts[1])
	case len(parts) == 3 && parts[0] == "profiles" && parts[2] == "bootstrap":
		s.renderBootstrapPage(w, r, parts[1])
	case len(parts) == 4 && parts[0] == "profiles" && parts[2] == "bundles":
		s.renderBundlePage(w, r, parts[1], parts[3])
	case path == "assets/style.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(web.Asset("style.css"))
	default:
		http.NotFound(w, r)
	}
}
