package api

import (
	"encoding/json"
	"log"
	"net/http"

	"audiobot-panel/backend/bot"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local development
	},
}

func (s *Server) HandleSessionStatusWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Subscribe to status events
	statusCh := s.manager.Subscribe()
	defer s.manager.Unsubscribe(statusCh)

	// Send current statuses immediately
	for _, room := range s.manager.GetRooms() {
		for _, info := range room.Bots {
			evt := bot.StatusEvent{
				BotID:  info.ID,
				Name:   info.Name,
				Status: info.Status,
				Error:  info.Error,
				RoomID: room.ID,
			}
			if err := conn.WriteJSON(evt); err != nil {
				return
			}
		}
	}

	// Set up read handler for ping/pong (keep-alive)
	conn.SetReadLimit(512)
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Forward status events
	for evt := range statusCh {
		data, err := json.Marshal(evt)
		if err != nil {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}
