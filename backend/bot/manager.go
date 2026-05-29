package bot

import (
	"context"
	"fmt"
	"sync"

	"audiobot-panel/data/names"

	"github.com/pion/webrtc/v4"
)

type BotStatus string

const (
	StatusConnecting BotStatus = "connecting"
	StatusActive     BotStatus = "active"
	StatusError      BotStatus = "error"
	StatusStopped    BotStatus = "stopped"
)

// StatusCallback is called by bot functions to report status changes.
type StatusCallback func(status BotStatus, errMsg string)

type BotInfo struct {
	ID      int       `json:"bot_id"`
	Name    string    `json:"name"`
	Status  BotStatus `json:"status"`
	Service string    `json:"service"`
	Error   string    `json:"error,omitempty"`
}

type BotInstance struct {
	info       BotInfo
	cancelFunc context.CancelFunc
}

type StatusEvent struct {
	BotID  int       `json:"bot_id"`
	Name   string    `json:"name"`
	Status BotStatus `json:"status"`
	Error  string    `json:"error,omitempty"`
	RoomID string    `json:"room_id,omitempty"`
}

// Room represents an active room session with its own set of bots.
type Room struct {
	ID         string
	Service    string
	RoomInput  string
	Bots       map[int]*BotInstance
	Active     bool
	opusFrames [][]byte
}

type BotManager struct {
	mu         sync.Mutex
	rooms      map[string]*Room
	statusSubs []chan StatusEvent
	storage    AudioStorage
	nextRoomID int
}

type AudioStorage interface {
	GetFilePath(fileID string) (string, error)
}

func NewBotManager(storage AudioStorage) *BotManager {
	return &BotManager{
		rooms:   make(map[string]*Room),
		storage: storage,
	}
}

// StartRoom creates a new room session and starts bots.
// Returns the room ID.
func (m *BotManager) StartRoom(service, roomInput string, botCount int, fileID string, loop bool) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if botCount < 1 || botCount > 3 {
		return "", fmt.Errorf("bot count must be 1-3, got %d", botCount)
	}

	filePath, err := m.storage.GetFilePath(fileID)
	if err != nil {
		return "", fmt.Errorf("get audio file: %w", err)
	}

	frames, err := LoadAudioFile(filePath)
	if err != nil {
		return "", fmt.Errorf("load audio: %w", err)
	}

	m.nextRoomID++
	roomID := fmt.Sprintf("room_%d", m.nextRoomID)

	room := &Room{
		ID:         roomID,
		Service:    service,
		RoomInput:  roomInput,
		Bots:       make(map[int]*BotInstance),
		Active:     true,
		opusFrames: frames,
	}
	m.rooms[roomID] = room

	for i := 1; i <= botCount; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		name := names.GenerateName()

		bot := &BotInstance{
			info: BotInfo{
				ID:      i,
				Name:    name,
				Status:  StatusConnecting,
				Service: service,
			},
			cancelFunc: cancel,
		}
		room.Bots[i] = bot

		go m.runBot(ctx, roomID, i, name, service, roomInput, loop, frames)
	}

	return roomID, nil
}

// StopRoom stops all bots in a specific room.
func (m *BotManager) StopRoom(roomID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, ok := m.rooms[roomID]
	if !ok {
		return fmt.Errorf("room not found: %s", roomID)
	}

	for _, bot := range room.Bots {
		if bot.cancelFunc != nil {
			bot.cancelFunc()
		}
		bot.info.Status = StatusStopped
		m.emitStatus(bot.info.ID, bot.info.Name, StatusStopped, "", roomID)
	}

	room.Active = false
	delete(m.rooms, roomID)
	return nil
}

// StopAll stops all rooms.
func (m *BotManager) StopAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for roomID, room := range m.rooms {
		for _, bot := range room.Bots {
			if bot.cancelFunc != nil {
				bot.cancelFunc()
			}
			bot.info.Status = StatusStopped
			m.emitStatus(bot.info.ID, bot.info.Name, StatusStopped, "", roomID)
		}
		room.Active = false
	}
	m.rooms = make(map[string]*Room)
	return nil
}

func (m *BotManager) runBot(ctx context.Context, roomID string, botID int, name, service, roomInput string, loop bool, frames [][]byte) {
	m.emitStatus(botID, name, StatusConnecting, "", roomID)

	// Callback for the bot to report status changes
	onStatus := func(status BotStatus, errMsg string) {
		m.mu.Lock()
		if room, ok := m.rooms[roomID]; ok {
			if bot, ok := room.Bots[botID]; ok {
				bot.info.Status = status
				bot.info.Error = errMsg
			}
		}
		m.mu.Unlock()
		m.emitStatus(botID, name, status, errMsg, roomID)
	}

	var err error
	switch service {
	case "jitsi":
		err = RunJitsiBot(ctx, botID, name, roomInput, frames, loop, onStatus)
	case "telemost":
		err = RunTelemostBot(ctx, botID, name, roomInput, frames, loop, onStatus)
	case "wbstream":
		err = RunWBStreamBot(ctx, botID, name, roomInput, frames, loop, onStatus)
	default:
		err = fmt.Errorf("unknown service: %s", service)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil && ctx.Err() == nil {
		if room, ok := m.rooms[roomID]; ok {
			if bot, ok := room.Bots[botID]; ok {
				bot.info.Status = StatusError
				bot.info.Error = err.Error()
			}
		}
		m.emitStatus(botID, name, StatusError, err.Error(), roomID)
	} else if ctx.Err() != nil {
		if room, ok := m.rooms[roomID]; ok {
			if bot, ok := room.Bots[botID]; ok {
				bot.info.Status = StatusStopped
			}
		}
		m.emitStatus(botID, name, StatusStopped, "", roomID)
	}
}

func (m *BotManager) emitStatus(botID int, name string, status BotStatus, errMsg string, roomID string) {
	evt := StatusEvent{
		BotID:  botID,
		Name:   name,
		Status: status,
		Error:  errMsg,
		RoomID: roomID,
	}
	for _, ch := range m.statusSubs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (m *BotManager) Subscribe() chan StatusEvent {
	ch := make(chan StatusEvent, 32)
	m.mu.Lock()
	m.statusSubs = append(m.statusSubs, ch)
	m.mu.Unlock()
	return ch
}

func (m *BotManager) Unsubscribe(ch chan StatusEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, sub := range m.statusSubs {
		if sub == ch {
			m.statusSubs = append(m.statusSubs[:i], m.statusSubs[i+1:]...)
			break
		}
	}
}

// GetRooms returns info about all active rooms.
func (m *BotManager) GetRooms() []RoomInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]RoomInfo, 0, len(m.rooms))
	for _, room := range m.rooms {
		ri := RoomInfo{
			ID:        room.ID,
			Service:   room.Service,
			RoomInput: room.RoomInput,
			Active:    room.Active,
		}
		for _, bot := range room.Bots {
			ri.Bots = append(ri.Bots, bot.info)
		}
		result = append(result, ri)
	}
	return result
}

type RoomInfo struct {
	ID        string    `json:"id"`
	Service   string    `json:"service"`
	RoomInput string    `json:"room_input"`
	Active    bool      `json:"active"`
	Bots      []BotInfo `json:"bots"`
}

func (m *BotManager) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.rooms) > 0
}

// CreateOpusTrack creates a sendonly Opus audio track for WebRTC.
func CreateOpusTrack() (*webrtc.TrackLocalStaticSample, error) {
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  1,
		},
		"audio", "audiobot",
	)
	if err != nil {
		return nil, fmt.Errorf("create track: %w", err)
	}
	return track, nil
}

// CreateSendOnlyPeerConnection creates a PeerConnection with a sendonly Opus track.
func CreateSendOnlyPeerConnection() (*webrtc.PeerConnection, *webrtc.TrackLocalStaticSample, error) {
	cfg := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}

	pc, err := webrtc.NewPeerConnection(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create peer connection: %w", err)
	}

	track, err := CreateOpusTrack()
	if err != nil {
		pc.Close()
		return nil, nil, err
	}

	if _, err := pc.AddTrack(track); err != nil {
		pc.Close()
		return nil, nil, fmt.Errorf("add track: %w", err)
	}

	return pc, track, nil
}

// CreatePlanBPeerConnection creates a PeerConnection with PlanB SDP semantics
// (required by Telemost/Goolom which sends PlanB SDP offers).
func CreatePlanBPeerConnection(iceServers []webrtc.ICEServer) (*webrtc.PeerConnection, error) {
	cfg := webrtc.Configuration{
		ICEServers:   iceServers,
		SDPSemantics: webrtc.SDPSemanticsPlanB,
	}

	pc, err := webrtc.NewPeerConnection(cfg)
	if err != nil {
		return nil, fmt.Errorf("create plan-b peer connection: %w", err)
	}

	return pc, nil
}
