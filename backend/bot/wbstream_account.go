package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"audiobot-panel/backend/config"
	"audiobot-panel/data/names"

	lksdk "github.com/livekit/server-sdk-go/v2"
)

// RunWBAccountKeeper runs a background loop that periodically joins
// every WB Stream room under a real user account (no audio published).
// This prevents rooms from being auto-closed by WB Stream.
func (m *BotManager) RunWBAccountKeeper(cfg *config.WBAccountConfig) {
	if cfg == nil {
		return
	}
	m.wbKeeperCfg = cfg
	go m.wbKeeperLoop()
}

// StopWBAccountKeeper stops the keeper goroutine.
func (m *BotManager) StopWBAccountKeeper() {
	select {
	case m.wbKeeperStop <- struct{}{}:
	default:
	}
}

func (m *BotManager) wbKeeperLoop() {
	if m.wbKeeperCfg == nil || !m.wbKeeperCfg.Enabled {
		return
	}
	interval := time.Duration(m.wbKeeperCfg.IntervalSec) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}
	stay := time.Duration(m.wbKeeperCfg.StayDurationSec) * time.Second
	if stay < 1*time.Second {
		stay = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on first tick, then wait interval.
	for {
		m.wbKeeperCycle(stay)
		select {
		case <-m.wbKeeperStop:
			log.Println("[wb-keeper] stopped")
			return
		case <-ticker.C:
		}
	}
}

func (m *BotManager) wbKeeperCycle(stay time.Duration) {
	rooms := m.getWBStreamRoomInputs()
	if len(rooms) == 0 {
		return
	}

	cfg := m.wbKeeperCfg
	token := cfg.AccessToken
	if token == "" && cfg.Cookies != "" {
		// Fallback: try to extract x_wbaas_token from raw cookie string.
		token = extractCookieValue(cfg.Cookies, "x_wbaas_token")
	}
	if token == "" {
		log.Println("[wb-keeper] no access token or cookie, skipping cycle")
		return
	}

	ua := cfg.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:151.0) Gecko/20100101 Firefox/151.0"
	}

	displayName := cfg.DisplayName
	if displayName == "" {
		displayName = names.GenerateName()
	}

	for _, roomInput := range rooms {
		roomID := parseWBStreamInput(roomInput)

		// Re-check room is still active before connecting.
		if !m.isRoomActive(roomInput) {
			log.Printf("[wb-keeper] skipping inactive room %s", roomID)
			continue
		}

		log.Printf("[wb-keeper] joining room %s as %s", roomID, displayName)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := m.wbJoinAndStay(ctx, roomID, token, ua, stay, displayName)
		cancel()

		if err != nil {
			log.Printf("[wb-keeper] room %s error: %v", roomID, err)
		} else {
			log.Printf("[wb-keeper] room %s done", roomID)
		}

		// Small delay between rooms.
		select {
		case <-m.wbKeeperStop:
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// isRoomActive checks whether a given room_input still has an active entry.
func (m *BotManager) isRoomActive(roomInput string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, room := range m.rooms {
		if room.RoomInput == roomInput && room.Active {
			return true
		}
	}
	return false
}

// wbJoinAndStay connects to a WB Stream room as a real user, stays for
// the requested duration, then disconnects.  No audio is published.
func (m *BotManager) wbJoinAndStay(ctx context.Context, roomID, token, ua string, stay time.Duration, displayName string) error {
	// Get LiveKit connection details via WB Stream REST API.
	// The room token already encodes the real-user identity and permissions,
	// so WB Stream tracks this participant through LiveKit events alone.
	connDetails, err := wbGetConnectionDetailsWithUA(ctx, roomID, token, displayName, ua)
	if err != nil {
		return fmt.Errorf("connection details: %w", err)
	}

	// Connect to LiveKit only.  Skip the separate WB Stream REST API join
	// to avoid a two-system participant record that can get out of sync
	// and create a ghost user that disrupts room routing.
	roomCallback := &lksdk.RoomCallback{}
	room := lksdk.NewRoom(roomCallback)
	defer room.Disconnect()

	if err := room.JoinWithToken(connDetails.ServerURL, connDetails.RoomToken, lksdk.WithAutoSubscribe(false)); err != nil {
		return fmt.Errorf("join livekit room: %w", err)
	}

	log.Printf("[wb-keeper] connected to %s, staying %v", roomID, stay)

	// Just stay — no track published.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(stay):
	}

	log.Printf("[wb-keeper] leaving %s", roomID)
	return nil
}

func (m *BotManager) getWBStreamRoomInputs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var inputs []string
	for _, room := range m.rooms {
		if room.Service == "wbstream" && room.Active {
			inputs = append(inputs, room.RoomInput)
		}
	}
	return inputs
}

// wbJoinRoomWithUA is a variant of wbJoinRoom that uses a custom User-Agent.
func wbJoinRoomWithUA(ctx context.Context, roomID, accessToken, ua string) error {
	u := fmt.Sprintf("%s/api-room/api/v1/room/%s/join", wbAPIBase, roomID)

	resp, err := wbSafePOSTWithUA(ctx, u, "application/json", []byte("{}"), map[string]string{
		"Authorization": "Bearer " + accessToken,
		"User-Agent":    ua,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

// wbGetConnectionDetailsWithUA is a variant of wbGetConnectionDetails with custom UA.
func wbGetConnectionDetailsWithUA(ctx context.Context, roomID, accessToken, displayName, ua string) (*wbConnDetails, error) {
	u := fmt.Sprintf("%s/api-room-manager/v2/room/%s/connection-details", wbAPIBase, roomID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("deviceType", "PARTICIPANT_DEVICE_TYPE_WEB_DESKTOP")
	q.Add("displayName", displayName)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", ua)

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: wbNewHTTPTransport(),
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	var result tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if result.ServerURL == "" || result.RoomToken == "" {
		return nil, fmt.Errorf("missing serverUrl or roomToken")
	}
	return &wbConnDetails{
		ServerURL: result.ServerURL,
		RoomToken: result.RoomToken,
	}, nil
}

// wbSafePOSTWithUA is like wbSafePOST but accepts a custom User-Agent header.
func wbSafePOSTWithUA(ctx context.Context, rawURL, contentType string, body []byte, headers map[string]string) (*http.Response, error) {
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: wbNewHTTPTransport(),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return client.Do(req)
}

// extractCookieValue parses a raw cookie string and returns the value for a given name.
func extractCookieValue(raw, name string) string {
	parts := strings.Split(raw, ";")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, name+"=") {
			return strings.TrimPrefix(p, name+"=")
		}
	}
	return ""
}
