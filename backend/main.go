package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"audiobot-panel/backend/api"
	"audiobot-panel/backend/auth"
	"audiobot-panel/backend/bot"
	"audiobot-panel/backend/config"
	"audiobot-panel/backend/storage"

	"github.com/gorilla/mux"
)

func findProjectRoot() string {
	// 1. Check CWD first — if go.mod exists, we're at project root
	if cwd, err := os.Getwd(); err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			return cwd
		}
	}

	// 2. Walk up from executable location to find go.mod
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exePath)
	for i := 0; i < 5; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// 3. Fallback to CWD
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

type spaHandler struct {
	staticDir  string
	fileServer http.Handler
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// Try to serve the exact file
	filePath := filepath.Join(h.staticDir, path)
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		h.fileServer.ServeHTTP(w, r)
		return
	}

	// Try .html extension
	if !strings.HasSuffix(path, ".html") {
		htmlPath := filepath.Join(h.staticDir, path+".html")
		if info, err := os.Stat(htmlPath); err == nil && !info.IsDir() {
			h.fileServer.ServeHTTP(w, r)
			return
		}
	}

	// Fallback to index.html for SPA routing
	r.URL.Path = "/"
	h.fileServer.ServeHTTP(w, r)
}

// compressedFileHandler serves pre-compressed .gz/.br files when the client
// sends Accept-Encoding. Falls back to the original handler otherwise.
type compressedFileHandler struct {
	handler   http.Handler
	staticDir string
}

func (h compressedFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accept := r.Header.Get("Accept-Encoding")
	if (strings.Contains(accept, "br") || strings.Contains(accept, "gzip")) && h.staticDir != "" {
		cleanPath := filepath.Clean(r.URL.Path)
		fsBase := filepath.Clean(h.staticDir)
		for _, enc := range []struct {
			ext    string
			header string
		}{
			{"br", "br"},
			{"gz", "gzip"},
		} {
			if !strings.Contains(accept, enc.header) {
				continue
			}
			fsPath := filepath.Join(h.staticDir, cleanPath+"."+enc.ext)
			// Prevent path traversal
			if !strings.HasPrefix(filepath.Clean(fsPath), fsBase) {
				continue
			}
			f, err := os.Open(fsPath)
			if err != nil {
				continue
			}
			defer f.Close()
			stat, err := f.Stat()
			if err != nil {
				continue
			}
			w.Header().Set("Content-Encoding", enc.header)
			w.Header().Set("Vary", "Accept-Encoding")
			// ServeContent uses the name param for Content-Type detection,
			// so passing the original filename gives correct MIME type (e.g. text/css).
			http.ServeContent(w, r, filepath.Base(cleanPath), stat.ModTime(), f)
			return
		}
	}
	h.handler.ServeHTTP(w, r)
}

func main() {
	projectRoot := findProjectRoot()
	log.Printf("Project root: %s", projectRoot)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	isElectron := os.Getenv("ELECTRON_MODE") == "1"
	configDir := filepath.Join(projectRoot, "config")
	audioDir := filepath.Join(projectRoot, "data", "audio")
	dataDir := filepath.Join(projectRoot, "data")

	// Initialize auth
	authCfg, err := auth.InitAuth(configDir, isElectron)
	if err != nil {
		log.Fatalf("auth init: %v", err)
	}

	// Initialize storage
	stor, err := storage.NewStorage(audioDir)
	if err != nil {
		log.Fatalf("storage init: %v", err)
	}

	// Initialize room config store
	roomStore, err := storage.NewRoomConfigStore(dataDir)
	if err != nil {
		log.Fatalf("room config store init: %v", err)
	}

	// Initialize account store
	accountStore, err := config.NewAccountStore(dataDir)
	if err != nil {
		log.Fatalf("account store init: %v", err)
	}

	// Initialize bot manager
	manager := bot.NewBotManager(stor, roomStore)

	// Start WB account keeper if enabled
	wbCfg := accountStore.Get()
	if wbCfg.Enabled {
		manager.RunWBAccountKeeper(&wbCfg)
	}

	// Create API server
	server := api.NewServer(manager, stor, accountStore, authCfg, isElectron)

	// Router
	r := mux.NewRouter()

	// API routes
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.Use(server.AuthMiddleware)

	apiRouter.HandleFunc("/audio/upload", server.HandleAudioUpload).Methods("POST")
	apiRouter.HandleFunc("/audio/list", server.HandleAudioList).Methods("GET")
	apiRouter.HandleFunc("/room/start", server.HandleRoomStart).Methods("POST")
	apiRouter.HandleFunc("/room/stop", server.HandleRoomStop).Methods("POST")
	apiRouter.HandleFunc("/room/delete", server.HandleRoomDelete).Methods("POST")
	apiRouter.HandleFunc("/room/restart", server.HandleRoomRestart).Methods("POST")
	apiRouter.HandleFunc("/room/update", server.HandleRoomUpdate).Methods("POST")
	apiRouter.HandleFunc("/room/start-from-config", server.HandleRoomStartFromConfig).Methods("POST")
	apiRouter.HandleFunc("/room/pause", server.HandleRoomPause).Methods("POST")
	apiRouter.HandleFunc("/room/resume", server.HandleRoomResume).Methods("POST")
	apiRouter.HandleFunc("/room/list", server.HandleRoomsList).Methods("GET")
	apiRouter.HandleFunc("/room/status", server.HandleSessionStatusWS).Methods("GET")
	apiRouter.HandleFunc("/wbstream/account", server.HandleWBAccountGet).Methods("GET")
	apiRouter.HandleFunc("/wbstream/account", server.HandleWBAccountSet).Methods("POST")
	apiRouter.HandleFunc("/wbstream/account/stop", server.HandleWBAccountStop).Methods("POST")

	// Auth endpoints (outside apiRouter to skip auth middleware)
	r.HandleFunc("/api/auth/mode", server.HandleAuthMode).Methods("GET")
	r.HandleFunc("/api/auth/login", server.HandleAuthLogin).Methods("POST")
	r.HandleFunc("/api/auth/check", server.HandleAuthCheck).Methods("GET")

	// Serve static frontend files (SPA with fallback to index.html)
	frontendPath := filepath.Join(projectRoot, "frontend", "build")
	if info, err := os.Stat(frontendPath); err == nil && info.IsDir() {
		log.Printf("Serving frontend from: %s", frontendPath)
		spa := &spaHandler{
			staticDir:  frontendPath,
			fileServer: http.FileServer(http.Dir(frontendPath)),
		}
		compressed := compressedFileHandler{handler: spa, staticDir: frontendPath}
		r.PathPrefix("/").Handler(compressed)
	} else {
		log.Printf("WARNING: frontend build not found at %s", frontendPath)
	}

	addr := fmt.Sprintf(":%s", port)
	fmt.Printf("PORT=%s\n", port)
	log.Printf("Starting AudioBot Panel server on %s (electron=%v)", addr, isElectron)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
