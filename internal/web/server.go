package web

import (
	"context"
	"crypto/subtle"
	"embed"
	"errors"
	"fmt"
	"go-port-forward/internal/config"
	"go-port-forward/internal/firewall"
	"go-port-forward/internal/forward"
	"go-port-forward/internal/logger"
	"io/fs"
	"net"
	"net/http"
	"path"
	"strings"
	"time"
)

// staticMIME maps file extensions to correct MIME types, derived from the
// official Nginx mime.types with additions for modern web formats.
// This bypasses mime.TypeByExtension which on Windows reads from the registry
// and may return text/plain, causing browsers to reject resources when
// X-Content-Type-Options: nosniff is set.
//
// Reference: https://github.com/nginx/nginx/blob/master/conf/mime.types
var staticMIME = map[string]string{
	// ── Text ──
	".html":     "text/html; charset=utf-8",
	".htm":      "text/html; charset=utf-8",
	".shtml":    "text/html; charset=utf-8",
	".css":      "text/css; charset=utf-8",
	".xml":      "text/xml; charset=utf-8",
	".csv":      "text/csv; charset=utf-8",
	".txt":      "text/plain; charset=utf-8",
	".vtt":      "text/vtt; charset=utf-8",
	".mml":      "text/mathml",
	".htc":      "text/x-component",
	".ics":      "text/calendar; charset=utf-8",
	".vcf":      "text/vcard; charset=utf-8",
	".md":       "text/markdown; charset=utf-8",
	".markdown": "text/markdown; charset=utf-8",
	".yaml":     "text/yaml; charset=utf-8",
	".yml":      "text/yaml; charset=utf-8",
	".jad":      "text/vnd.sun.j2me.app-descriptor",
	".wml":      "text/vnd.wap.wml",

	// ── JavaScript / JSON / WASM ──
	".js":          "text/javascript; charset=utf-8",
	".mjs":         "text/javascript; charset=utf-8",
	".json":        "application/json; charset=utf-8",
	".jsonld":      "application/ld+json; charset=utf-8",
	".geojson":     "application/geo+json; charset=utf-8",
	".map":         "application/json",
	".webmanifest": "application/manifest+json",
	".wasm":        "application/wasm",

	// ── XML-based ──
	".atom":  "application/atom+xml",
	".rss":   "application/rss+xml",
	".xhtml": "application/xhtml+xml",
	".xspf":  "application/xspf+xml",
	".xslt":  "application/xslt+xml",
	".kml":   "application/vnd.google-earth.kml+xml",
	".kmz":   "application/vnd.google-earth.kmz",

	// ── Images ──
	".apng": "image/apng",
	".avif": "image/avif",
	".bmp":  "image/x-ms-bmp",
	".cur":  "image/x-icon",
	".gif":  "image/gif",
	".heic": "image/heic",
	".heif": "image/heif",
	".ico":  "image/x-icon",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".jng":  "image/x-jng",
	".jxl":  "image/jxl",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".svgz": "image/svg+xml",
	".tif":  "image/tiff",
	".tiff": "image/tiff",
	".wbmp": "image/vnd.wap.wbmp",
	".webp": "image/webp",

	// ── Fonts ──
	".eot":   "application/vnd.ms-fontobject",
	".otf":   "font/otf",
	".ttf":   "font/ttf",
	".ttc":   "font/collection",
	".woff":  "font/woff",
	".woff2": "font/woff2",

	// ── Audio ──
	".aac":  "audio/aac",
	".flac": "audio/flac",
	".kar":  "audio/midi",
	".m4a":  "audio/x-m4a",
	".mid":  "audio/midi",
	".midi": "audio/midi",
	".mp3":  "audio/mpeg",
	".oga":  "audio/ogg",
	".ogg":  "audio/ogg",
	".opus": "audio/opus",
	".ra":   "audio/x-realaudio",
	".wav":  "audio/wav",
	".weba": "audio/webm",

	// ── Video ──
	".3gp":  "video/3gpp",
	".3gpp": "video/3gpp",
	".asf":  "video/x-ms-asf",
	".asx":  "video/x-ms-asf",
	".avi":  "video/x-msvideo",
	".flv":  "video/x-flv",
	".m4v":  "video/x-m4v",
	".mkv":  "video/x-matroska",
	".mng":  "video/x-mng",
	".mov":  "video/quicktime",
	".mp4":  "video/mp4",
	".mpeg": "video/mpeg",
	".mpg":  "video/mpeg",
	".ogv":  "video/ogg",
	".ts":   "video/mp2t",
	".webm": "video/webm",
	".wmv":  "video/x-ms-wmv",

	// ── Archives / Packages ──
	".7z":  "application/x-7z-compressed",
	".br":  "application/brotli",
	".bz2": "application/x-bzip2",
	".deb": "application/vnd.debian.binary-package",
	".dmg": "application/x-apple-diskimage",
	".gz":  "application/gzip",
	".iso": "application/x-iso9660-image",
	".rar": "application/x-rar-compressed",
	".rpm": "application/x-redhat-package-manager",
	".tar": "application/x-tar",
	".tgz": "application/gzip",
	".xz":  "application/x-xz",
	".zip": "application/zip",
	".zst": "application/zstd",

	// ── Documents / Office ──
	".doc":  "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".epub": "application/epub+zip",
	".odg":  "application/vnd.oasis.opendocument.graphics",
	".odp":  "application/vnd.oasis.opendocument.presentation",
	".ods":  "application/vnd.oasis.opendocument.spreadsheet",
	".odt":  "application/vnd.oasis.opendocument.text",
	".pdf":  "application/pdf",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".rtf":  "application/rtf",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",

	// ── Security / Certificates ──
	".crt": "application/x-x509-ca-cert",
	".der": "application/x-x509-ca-cert",
	".p7s": "application/pkcs7-signature",
	".pem": "application/x-x509-ca-cert",

	// ── Binary / Executable ──
	".bin": "application/octet-stream",
	".dll": "application/octet-stream",
	".exe": "application/octet-stream",
	".msi": "application/octet-stream",

	// ── Other Application ──
	".ai":      "application/postscript",
	".apk":     "application/vnd.android.package-archive",
	".cbor":    "application/cbor",
	".cco":     "application/x-cocoa",
	".ear":     "application/java-archive",
	".eps":     "application/postscript",
	".hqx":     "application/mac-binhex40",
	".jar":     "application/java-archive",
	".jardiff": "application/x-java-archive-diff",
	".jnlp":    "application/x-java-jnlp-file",
	".m3u8":    "application/vnd.apple.mpegurl",
	".mpd":     "application/dash+xml",
	".pl":      "application/x-perl",
	".pm":      "application/x-perl",
	".ps":      "application/postscript",
	".run":     "application/x-makeself",
	".sea":     "application/x-sea",
	".sit":     "application/x-stuffit",
	".sqlite":  "application/vnd.sqlite3",
	".swf":     "application/x-shockwave-flash",
	".torrent": "application/x-bittorrent",
	".war":     "application/java-archive",
	".wmlc":    "application/vnd.wap.wmlc",
	".xpi":     "application/x-xpinstall",
}

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
		if err := s.httpSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	mux.HandleFunc("GET /api/dashboard", h.dashboard)
	mux.HandleFunc("GET /api/diagnostics", h.diagnostics)
	mux.HandleFunc("GET /api/stats", h.globalStats)

	// WSL
	mux.HandleFunc("GET /api/wsl/capability", h.wslCapability)
	mux.HandleFunc("GET /api/wsl/distros", h.wslListDistros)
	mux.HandleFunc("GET /api/wsl/ports/{distro}", h.wslListPorts)
	mux.HandleFunc("POST /api/wsl/import", h.wslImport)

	// Embedded SPA — wrap with fixMIME to guarantee correct Content-Type
	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", fixMIME(http.FileServer(http.FS(staticFS))))
}

// fixMIME wraps a handler to force correct Content-Type for known static file
// extensions. This bypasses http.FileServer's reliance on mime.TypeByExtension
// (which reads from the Windows registry and may return text/plain).
func fixMIME(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct, ok := staticMIME[strings.ToLower(path.Ext(r.URL.Path))]; ok {
			next.ServeHTTP(&mimeLockedWriter{ResponseWriter: w, contentType: ct}, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// mimeLockedWriter wraps http.ResponseWriter and forces the Content-Type
// header on the first WriteHeader/Write, preventing http.FileServer from
// overwriting it with an OS-derived value.
type mimeLockedWriter struct {
	http.ResponseWriter
	contentType string
	wroteHeader bool
}

func (w *mimeLockedWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.ResponseWriter.Header().Set("Content-Type", w.contentType)
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *mimeLockedWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.ResponseWriter.Header().Set("Content-Type", w.contentType)
	}
	return w.ResponseWriter.Write(b)
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
