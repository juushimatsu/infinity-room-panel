package bot

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	lksdk "github.com/livekit/server-sdk-go/v2"
	webrtcv3 "github.com/pion/webrtc/v3"
	media3 "github.com/pion/webrtc/v3/pkg/media"
)

// wbAPIBase is the base URL for WB Stream API.
const wbAPIBase = "https://stream.wb.ru"

// RunWBStreamBot connects a bot to a WB Stream room via LiveKit
// and streams Opus audio frames.
// Based on olcrtc/internal/auth/wbstream and internal/engine/livekit reference.
func RunWBStreamBot(ctx context.Context, botID int, name, roomInput string, opusFrames [][]byte, loop bool, onStatus StatusCallback) error {
	roomID := parseWBStreamInput(roomInput)
	if name == "" {
		name = fmt.Sprintf("Bot%d", botID)
	}

	accessToken, err := wbGuestRegister(ctx, name)
	if err != nil {
		return fmt.Errorf("guest register: %w", err)
	}

	if err := wbJoinRoom(ctx, roomID, accessToken); err != nil {
		return fmt.Errorf("join room: %w", err)
	}

	connDetails, err := wbGetConnectionDetails(ctx, roomID, accessToken, name)
	if err != nil {
		return fmt.Errorf("get connection details: %w", err)
	}

	roomCallback := &lksdk.RoomCallback{}
	room := lksdk.NewRoom(roomCallback)
	defer room.Disconnect()

	if err := room.JoinWithToken(connDetails.ServerURL, connDetails.RoomToken, lksdk.WithAutoSubscribe(false)); err != nil {
		return fmt.Errorf("join livekit room: %w", err)
	}

	audioTrack, err := lksdk.NewLocalTrack(webrtcv3.RTPCodecCapability{
		MimeType:  webrtcv3.MimeTypeOpus,
		ClockRate: 48000,
		Channels:  1,
	})
	if err != nil {
		return fmt.Errorf("create local track: %w", err)
	}

	if _, err := room.LocalParticipant.PublishTrack(audioTrack, &lksdk.TrackPublicationOptions{
		Name: fmt.Sprintf("bot%d-audio", botID),
	}); err != nil {
		return fmt.Errorf("publish audio track: %w", err)
	}

	bindCh := make(chan struct{}, 1)
	audioTrack.OnBind(func() {
		select {
		case bindCh <- struct{}{}:
		default:
		}
	})

	select {
	case <-bindCh:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("track bind timeout")
	case <-ctx.Done():
		return ctx.Err()
	}

	if onStatus != nil {
		onStatus(StatusActive, "")
	}

	iter := NewFrameIterator(opusFrames, loop)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			frame, ok := iter.Next()
			if !ok {
				return nil
			}
			sample := media3.Sample{
				Data:     frame,
				Duration: 20 * time.Millisecond,
			}
			if err := audioTrack.WriteSample(sample, nil); err != nil {
				return fmt.Errorf("write sample: %w", err)
			}
		}
	}
}

type wbConnDetails struct {
	ServerURL string
	RoomToken string
}

// parseWBStreamInput extracts the room ID from user input.
// Accepts either a raw room ID or a URL like https://stream.wb.ru/streams/ROOM_ID
func parseWBStreamInput(input string) string {
	input = strings.TrimSpace(input)
	// If it's a URL, extract the last path segment as room ID
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		parts := strings.Split(input, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			if parts[i] != "" {
				return parts[i]
			}
		}
	}
	return input
}

// guestRegisterRequest matches the olcrtc/internal/auth/wbstream/api.go struct exactly.
// The WB Stream API uses proto3 JSON mapping (camelCase) — confirmed by olcrtc and Python PoC.
type guestRegisterRequest struct {
	DisplayName string `json:"displayName"`
	Device      struct {
		DeviceName string `json:"deviceName"`
		DeviceType string `json:"deviceType"`
	} `json:"device"`
}

type guestRegisterResponse struct {
	AccessToken string `json:"accessToken"`
}

type tokenResponse struct {
	RoomToken string `json:"roomToken"`
	ServerURL string `json:"serverUrl"`
}

// wbNewHTTPTransport creates an HTTP transport matching olcrtc protect.NewHTTPTransport.
// Uses proper TLS config (MinVersion TLS12, no InsecureSkipVerify), HTTP/2, and sane timeouts.
func wbNewHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}
}

// wbSafePOST performs a POST request with proper redirect handling.
// Go's http.Client changes POST→GET and drops the body on 301/302 redirects.
// This function manually follows redirects while preserving the POST method and body.
//
// Uses bytes.NewReader instead of bytes.NewBuffer so that Go can determine
// Content-Length explicitly (NewReader implements io.Seeker). This is critical
// for gRPC-gateway servers which may not support Transfer-Encoding: chunked.
func wbSafePOST(ctx context.Context, rawURL, contentType string, body []byte, headers map[string]string) (*http.Response, error) {
	noRedirectClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: wbNewHTTPTransport(),
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	const maxRedirects = 5
	currentURL := rawURL

	for i := 0; i < maxRedirects; i++ {
		// Use bytes.NewReader — it implements io.Seeker, so Go sets
		// Content-Length header explicitly instead of using chunked encoding.
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, currentURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Linux x86_64)")
		// Set Content-Length explicitly to ensure body is not sent as chunked
		req.ContentLength = int64(len(body))
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		log.Printf("[wbstream] POST %s ContentLength=%d headers=%v", currentURL, req.ContentLength, req.Header)

		resp, err := noRedirectClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("do request: %w", err)
		}

		log.Printf("[wbstream] POST response status=%d Location=%s headers=%v",
			resp.StatusCode, resp.Header.Get("Location"), resp.Header)

		// Not a redirect — return the response
		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			return resp, nil
		}

		// It's a redirect — follow it manually, preserving POST method and body
		loc := resp.Header.Get("Location")
		_ = resp.Body.Close()
		if loc == "" {
			return nil, fmt.Errorf("redirect with no Location header")
		}

		// Resolve relative URLs
		if !strings.HasPrefix(loc, "http") {
			base, err := url.Parse(currentURL)
			if err != nil {
				return nil, fmt.Errorf("parse base URL %s: %w", currentURL, err)
			}
			ref, err := url.Parse(loc)
			if err != nil {
				return nil, fmt.Errorf("parse redirect URL %s: %w", loc, err)
			}
			loc = base.ResolveReference(ref).String()
		}

		log.Printf("[wbstream] POST %d redirect: %s → %s (resending body len=%d)", resp.StatusCode, currentURL, loc, len(body))
		currentURL = loc
	}

	return nil, fmt.Errorf("too many redirects (>%d)", maxRedirects)
}

func wbGuestRegister(ctx context.Context, displayName string) (string, error) {
	u := wbAPIBase + "/auth/api/v1/auth/user/guest-register"

	reqBody := guestRegisterRequest{
		DisplayName: displayName,
	}
	reqBody.Device.DeviceName = "Linux"
	reqBody.Device.DeviceType = "PARTICIPANT_DEVICE_TYPE_WEB_DESKTOP"

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	log.Printf("[wbstream] guest-register name=%q body=%s", displayName, string(body))

	resp, err := wbSafePOST(ctx, u, "application/json", body, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[wbstream] guest-register status=%d body=%s", resp.StatusCode, string(respBody))

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return "", fmt.Errorf("guest register status %d: %s", resp.StatusCode, string(respBody))
	}

	var result guestRegisterResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response: %s", string(respBody))
	}
	return result.AccessToken, nil
}

func wbJoinRoom(ctx context.Context, roomID, accessToken string) error {
	u := fmt.Sprintf("%s/api-room/api/v1/room/%s/join", wbAPIBase, roomID)

	// Body must be empty JSON "{}" — matching olcrtc/internal/auth/wbstream/api.go
	resp, err := wbSafePOST(ctx, u, "application/json", []byte("{}"), map[string]string{
		"Authorization": "Bearer " + accessToken,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[wbstream] join-room status=%d body=%s", resp.StatusCode, string(respBody))

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return fmt.Errorf("join room status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func wbGetConnectionDetails(ctx context.Context, roomID, accessToken, displayName string) (*wbConnDetails, error) {
	u := fmt.Sprintf("%s/api-room-manager/v2/room/%s/connection-details", wbAPIBase, roomID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	// Query params matching olcrtc/internal/auth/wbstream/api.go
	q := req.URL.Query()
	q.Add("deviceType", "PARTICIPANT_DEVICE_TYPE_WEB_DESKTOP")
	q.Add("displayName", displayName)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux x86_64)")

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: wbNewHTTPTransport(),
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[wbstream] connection-details status=%d body=%s", resp.StatusCode, string(respBody))

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("connection details status %d: %s", resp.StatusCode, string(respBody))
	}

	var result tokenResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if result.ServerURL == "" || result.RoomToken == "" {
		return nil, fmt.Errorf("missing serverUrl or roomToken in response: %s", string(respBody))
	}

	return &wbConnDetails{
		ServerURL: result.ServerURL,
		RoomToken: result.RoomToken,
	}, nil
}
