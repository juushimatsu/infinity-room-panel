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

	// Initialize bot manager
	manager := bot.NewBotManager(stor)

	// Create API server
	server := api.NewServer(manager, stor, authCfg, isElectron)

	// Router
	r := mux.NewRouter()

	// API routes
	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.Use(server.AuthMiddleware)

	apiRouter.HandleFunc("/audio/upload", server.HandleAudioUpload).Methods("POST")
	apiRouter.HandleFunc("/audio/list", server.HandleAudioList).Methods("GET")
	apiRouter.HandleFunc("/room/start", server.HandleRoomStart).Methods("POST")
	apiRouter.HandleFunc("/room/stop", server.HandleRoomStop).Methods("POST")
	apiRouter.HandleFunc("/room/list", server.HandleRoomsList).Methods("GET")
	apiRouter.HandleFunc("/room/status", server.HandleSessionStatusWS).Methods("GET")

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
		r.PathPrefix("/").Handler(spa)
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
