package web

import (
	"context"
	"crypto/subtle"
	"embed"
	"fmt"
	"go-port-forward/internal/config"
	"go-port-forward/internal/firewall"
	"go-port-forward/internal/forward"
	"go-port-forward/internal/logger"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"
)

//go:embed static
var staticFiles embed.FS

// Server is the HTTP API + UI server.
type Server struct {
	fw      firewall.Manager
	manager *forward.Manager
	httpSrv *http.Server
	cfg     config.WebConfig
}

// New creates a configured Server.
func New(cfg config.WebConfig, mgr *forward.Manager, fw firewall.Manager) *Server {
	return &Server{cfg: cfg, manager: mgr, fw: fw}
}

// Start begins listening on the configured address (non-blocking).
func (s *Server) Start() error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      s.middlewareChain(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.S.Infow("Web UI listening", "addr", "http://"+addr)
		if err := s.httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.S.Errorw("HTTP server error", "err", err)
		}
	}()
	return nil
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	h := &handler{mgr: s.manager, fw: s.fw}

	// REST API
	mux.HandleFunc("GET /api/rules", h.listRules)
	mux.HandleFunc("POST /api/rules", h.createRule)
	mux.HandleFunc("GET /api/rules/{id}", h.getRule)
	mux.HandleFunc("PUT /api/rules/{id}", h.updateRule)
	mux.HandleFunc("DELETE /api/rules/{id}", h.deleteRule)
	mux.HandleFunc("PUT /api/rules/{id}/toggle", h.toggleRule)
	mux.HandleFunc("GET /api/stats", h.globalStats)

	// WSL
	mux.HandleFunc("GET /api/wsl/distros", h.wslListDistros)
	mux.HandleFunc("GET /api/wsl/ports/{distro}", h.wslListPorts)
	mux.HandleFunc("POST /api/wsl/import", h.wslImport)

	// Embedded SPA
	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(staticFS)))
}

// middlewareChain wraps the mux with logging and optional basic auth.
func (s *Server) middlewareChain(next http.Handler) http.Handler {
	// Basic auth
	if s.cfg.Username != "" {
		next = basicAuth(s.cfg.Username, s.cfg.Password, next)
	}
	// Request logger
	next = requestLogger(next)
	next = securityHeaders(next)
	return next
}

func basicAuth(user, pass string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		userOK := subtle.ConstantTimeCompare([]byte(u), []byte(user)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(p), []byte(pass)) == 1
		if !ok || !userOK || !passOK {
			w.Header().Set("WWW-Authenticate", `Basic realm="go-port-forward"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.S.Debugw("HTTP", "method", r.Method, "path", r.URL.Path,
			"duration", time.Since(start).String())
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
