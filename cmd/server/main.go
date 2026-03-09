package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultDataFile = "data/history.json"
	maxEntries      = 500
	defaultLimit    = 20
)

type historyEntry struct {
	ID           string    `json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	Mode         string    `json:"mode"`
	ServerName   string    `json:"serverName,omitempty"`
	ClientIP     string    `json:"clientIp,omitempty"`
	DownloadMbps float64   `json:"downloadMbps"`
	UploadMbps   float64   `json:"uploadMbps"`
	PingMs       float64   `json:"pingMs"`
	JitterMs     float64   `json:"jitterMs"`
	DownloadMB   float64   `json:"downloadMB"`
	UploadMB     float64   `json:"uploadMB"`
	UserAgent    string    `json:"userAgent,omitempty"`
}

type historyPayload struct {
	Mode         string  `json:"mode"`
	ServerName   string  `json:"serverName"`
	ClientIP     string  `json:"clientIp"`
	DownloadMbps float64 `json:"downloadMbps"`
	UploadMbps   float64 `json:"uploadMbps"`
	PingMs       float64 `json:"pingMs"`
	JitterMs     float64 `json:"jitterMs"`
	DownloadMB   float64 `json:"downloadMB"`
	UploadMB     float64 `json:"uploadMB"`
	UserAgent    string  `json:"userAgent"`
}

type historyStore struct {
	mu      sync.RWMutex
	path    string
	entries []historyEntry
}

func main() {
	addr := flag.String("addr", ":3000", "HTTP listen address")
	root := flag.String("root", ".", "project root containing static assets")
	dataFile := flag.String("data-file", defaultDataFile, "path to history JSON file")
	flag.Parse()

	store, err := newHistoryStore(*dataFile)
	if err != nil {
		log.Fatalf("init history store: %v", err)
	}

	server := newServer(*root, store)

	log.Printf("OpenSpeedTestX server listening on %s", *addr)
	log.Printf("Serving static assets from %s", filepath.Clean(*root))
	log.Printf("Persisting history in %s", filepath.Clean(*dataFile))

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           server,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Fatal(httpServer.ListenAndServe())
}

func newServer(root string, store *historyStore) http.Handler {
	mux := http.NewServeMux()

	assetsDir := filepath.Join(root, "assets")
	indexPath := filepath.Join(root, "index.html")
	hostedPath := filepath.Join(root, "hosted.html")
	downloadPath := filepath.Join(root, "downloading")
	licensePath := filepath.Join(root, "License.md")

	mux.Handle("/assets/", cacheControl(http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir)))))
	mux.HandleFunc("/api/history", historyHandler(store))
	mux.HandleFunc("/upload", uploadHandler)
	mux.HandleFunc("/downloading", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, downloadPath)
	})
	mux.HandleFunc("/hosted.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, hostedPath)
	})
	mux.HandleFunc("/License.md", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, licensePath)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, indexPath)
	})

	return withLogging(mux)
}

func cacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".js") || strings.HasSuffix(r.URL.Path, ".css") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, OPTIONS")

	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
	case http.MethodGet, http.MethodHead:
		w.WriteHeader(http.StatusOK)
	case http.MethodPost:
		defer r.Body.Close()
		if _, err := io.Copy(io.Discard, r.Body); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "failed to read upload body", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.Header().Set("Allow", "GET, HEAD, POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func historyHandler(store *historyStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Cache-Control", "no-store")

		switch r.Method {
		case http.MethodOptions:
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			limit := defaultLimit
			if raw := r.URL.Query().Get("limit"); raw != "" {
				parsed, err := strconv.Atoi(raw)
				if err != nil || parsed < 1 {
					http.Error(w, "invalid limit", http.StatusBadRequest)
					return
				}
				limit = parsed
			}
			writeJSON(w, http.StatusOK, store.List(limit))
		case http.MethodPost:
			defer r.Body.Close()
			var payload historyPayload
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&payload); err != nil {
				http.Error(w, "invalid JSON payload", http.StatusBadRequest)
				return
			}
			entry, err := store.Add(payload, resolveClientIP(payload.ClientIP, clientIPFromRequest(r)))
			if err != nil {
				http.Error(w, "failed to save history", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusCreated, entry)
		case http.MethodDelete:
			id := strings.TrimSpace(r.URL.Query().Get("id"))
			if id == "" {
				http.Error(w, "missing id", http.StatusBadRequest)
				return
			}
			if err := store.Delete(id); err != nil {
				http.Error(w, "failed to delete history", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Header().Set("Allow", "GET, POST, DELETE, OPTIONS")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func newHistoryStore(path string) (*historyStore, error) {
	store := &historyStore{path: path}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *historyStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.entries = []historyEntry{}
		return nil
	}
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		s.entries = []historyEntry{}
		return nil
	}

	var entries []historyEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return err
	}
	s.entries = entries
	return nil
}

func (s *historyStore) List(limit int) []historyEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.entries) {
		limit = len(s.entries)
	}
	result := make([]historyEntry, limit)
	copy(result, s.entries[:limit])
	return result
}

func (s *historyStore) Add(payload historyPayload, clientIP string) (historyEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := historyEntry{
		ID:           strconv.FormatInt(time.Now().UnixNano(), 10),
		CreatedAt:    time.Now().UTC(),
		Mode:         normalizeMode(payload.Mode),
		ServerName:   strings.TrimSpace(payload.ServerName),
		ClientIP:     strings.TrimSpace(clientIP),
		DownloadMbps: round(payload.DownloadMbps, 3),
		UploadMbps:   round(payload.UploadMbps, 3),
		PingMs:       round(payload.PingMs, 1),
		JitterMs:     round(payload.JitterMs, 1),
		DownloadMB:   round(payload.DownloadMB, 3),
		UploadMB:     round(payload.UploadMB, 3),
		UserAgent:    strings.TrimSpace(payload.UserAgent),
	}

	s.entries = append([]historyEntry{entry}, s.entries...)
	if len(s.entries) > maxEntries {
		s.entries = s.entries[:maxEntries]
	}
	if err := s.persistLocked(); err != nil {
		return historyEntry{}, err
	}
	return entry, nil
}

func (s *historyStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = []historyEntry{}
	return s.persistLocked()
}

func (s *historyStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}

	filtered := s.entries[:0]
	removed := false
	for _, entry := range s.entries {
		if entry.ID == id {
			removed = true
			continue
		}
		filtered = append(filtered, entry)
	}

	if !removed {
		return nil
	}

	s.entries = filtered
	return s.persistLocked()
}

func (s *historyStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	tmpPath := s.path + ".tmp"
	raw, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')

	if err := os.WriteFile(tmpPath, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

func normalizeMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "download":
		return "download"
	case "upload":
		return "upload"
	case "ping":
		return "ping"
	default:
		return "full"
	}
}

func clientIPFromRequest(r *http.Request) string {
	forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwardedFor != "" {
		first := strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
		if first != "" {
			return first
		}
	}

	realIP := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if realIP != "" {
		return realIP
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func resolveClientIP(browserIP, requestIP string) string {
	browserIP = normalizeIP(browserIP)
	requestIP = normalizeIP(requestIP)

	if requestIP != "" && !isLoopbackIP(requestIP) {
		return requestIP
	}
	if browserIP != "" && !isLoopbackIP(browserIP) {
		return browserIP
	}
	if requestIP != "" {
		return requestIP
	}
	return browserIP
}

func normalizeIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, ",") {
		value = strings.TrimSpace(strings.Split(value, ",")[0])
	}
	if parsed := net.ParseIP(value); parsed != nil {
		return parsed.String()
	}
	return value
}

func isLoopbackIP(value string) bool {
	parsed := net.ParseIP(strings.TrimSpace(value))
	if parsed == nil {
		return value == "localhost"
	}
	return parsed.IsLoopback() || parsed.IsUnspecified()
}

func round(value float64, precision int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	pow := math.Pow10(precision)
	return math.Round(value*pow) / pow
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(start).Round(time.Millisecond))
	})
}
