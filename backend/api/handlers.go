package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"audiobot-panel/backend/auth"
	"audiobot-panel/backend/bot"
	"audiobot-panel/backend/config"
	"audiobot-panel/backend/storage"
)

type Server struct {
	manager      *bot.BotManager
	storage      *storage.Storage
	accountStore *config.AccountStore
	authCfg      *auth.AuthConfig
	isElectron   bool
}

func NewServer(manager *bot.BotManager, storage *storage.Storage, accountStore *config.AccountStore, authCfg *auth.AuthConfig, isElectron bool) *Server {
	return &Server{
		manager:      manager,
		storage:      storage,
		accountStore: accountStore,
		authCfg:      authCfg,
		isElectron:   isElectron,
	}
}

type startRequest struct {
	Service   string `json:"service"`
	RoomInput string `json:"room_input"`
	BotCount  int    `json:"bot_count"`
	FileID    string `json:"file_id"`
	Loop      bool   `json:"loop"`
}

type uploadResponse struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// AuthMiddleware checks JWT token for non-Electron mode.
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.isElectron {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for login endpoint
		if r.URL.Path == "/api/auth/login" {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing token")
			return
		}

		valid, err := auth.ValidateToken(token, s.authCfg.JWTSecret)
		if err != nil || !valid {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) HandleAudioUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no file in request")
		return
	}
	defer file.Close()

	fileID, err := s.storage.UploadFile(header.Filename, file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, uploadResponse{
		ID:       fileID,
		Filename: header.Filename,
	})
}

func (s *Server) HandleAudioList(w http.ResponseWriter, r *http.Request) {
	files := s.storage.ListFiles()
	writeJSON(w, http.StatusOK, files)
}

func (s *Server) HandleRoomStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req startRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.Service != "jitsi" && req.Service != "telemost" && req.Service != "wbstream" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid service: %s", req.Service))
		return
	}

	if req.RoomInput == "" {
		writeError(w, http.StatusBadRequest, "room_input is required")
		return
	}

	if req.BotCount < 1 || req.BotCount > 3 {
		writeError(w, http.StatusBadRequest, "bot_count must be 1-3")
		return
	}

	if req.FileID == "" {
		writeError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	roomID, err := s.manager.StartRoom(req.Service, req.RoomInput, req.BotCount, req.FileID, req.Loop)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"room_id": roomID})
}

func (s *Server) HandleRoomStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.RoomID == "" {
		writeError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	if err := s.manager.StopRoom(req.RoomID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) HandleRoomPause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.RoomID == "" {
		writeError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	if err := s.manager.PauseRoom(req.RoomID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (s *Server) HandleRoomResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.RoomID == "" {
		writeError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	if err := s.manager.ResumeRoom(req.RoomID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

func (s *Server) HandleRoomDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.RoomID == "" {
		writeError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	if err := s.manager.DeleteRoom(req.RoomID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (s *Server) HandleRoomRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.RoomID == "" {
		writeError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	if err := s.manager.RestartRoom(req.RoomID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

func (s *Server) HandleRoomUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID    string `json:"room_id"`
		Service   string `json:"service"`
		RoomInput string `json:"room_input"`
		BotCount  int    `json:"bot_count"`
		FileID    string `json:"file_id"`
		Loop      bool   `json:"loop"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.RoomID == "" {
		writeError(w, http.StatusBadRequest, "room_id is required")
		return
	}
	if req.Service != "jitsi" && req.Service != "telemost" && req.Service != "wbstream" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid service: %s", req.Service))
		return
	}
	if req.RoomInput == "" {
		writeError(w, http.StatusBadRequest, "room_input is required")
		return
	}
	if req.BotCount < 1 || req.BotCount > 3 {
		writeError(w, http.StatusBadRequest, "bot_count must be 1-3")
		return
	}
	if req.FileID == "" {
		writeError(w, http.StatusBadRequest, "file_id is required")
		return
	}

	if err := s.manager.UpdateRoomConfig(req.RoomID, req.Service, req.RoomInput, req.BotCount, req.FileID, req.Loop); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) HandleRoomStartFromConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RoomID string `json:"room_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.RoomID == "" {
		writeError(w, http.StatusBadRequest, "room_id is required")
		return
	}

	cfg, ok := s.manager.GetRoomConfig(req.RoomID)
	if !ok {
		writeError(w, http.StatusBadRequest, "room config not found")
		return
	}

	roomID, err := s.manager.StartRoom(cfg.Service, cfg.RoomInput, cfg.BotCount, cfg.FileID, cfg.Loop)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"room_id": roomID})
}

func (s *Server) HandleRoomsList(w http.ResponseWriter, r *http.Request) {
	rooms := s.manager.GetRooms()
	writeJSON(w, http.StatusOK, rooms)
}

func (s *Server) HandleWBAccountGet(w http.ResponseWriter, r *http.Request) {
	if s.accountStore == nil {
		writeJSON(w, http.StatusOK, config.WBAccountConfig{})
		return
	}
	cfg := s.accountStore.Get()
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) HandleWBAccountSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.accountStore == nil {
		writeError(w, http.StatusInternalServerError, "account store not available")
		return
	}

	var req config.WBAccountConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if req.IntervalSec < 10 {
		req.IntervalSec = 60
	}
	if req.StayDurationSec < 1 {
		req.StayDurationSec = 5
	}

	// Restart keeper with new config.
	s.manager.StopWBAccountKeeper()
	if err := s.accountStore.Set(req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.Enabled {
		s.manager.RunWBAccountKeeper(&req)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (s *Server) HandleWBAccountStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	s.manager.StopWBAccountKeeper()
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) HandleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if !auth.CheckPassword(req.Password, s.authCfg.HashedPassword) {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	token, err := auth.GenerateToken(s.authCfg.JWTSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token generation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) HandleAuthMode(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"electron": s.isElectron})
}

func (s *Server) HandleAuthCheck(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	if token == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"valid": false})
		return
	}

	valid, err := auth.ValidateToken(token, s.authCfg.JWTSecret)
	if err != nil || !valid {
		writeJSON(w, http.StatusOK, map[string]bool{"valid": false})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"valid": true})
}
