package bot

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v4"
)

// RunTelemostBot connects a bot to a Yandex Telemost conference
// and streams Opus audio frames via the Goolom signaling protocol.
// Based on olcrtc internal/auth/telemost and internal/engine/goolom reference.
func RunTelemostBot(ctx context.Context, botID int, name, roomInput string, opusFrames [][]byte, loop bool, onStatus StatusCallback) error {
	roomURL, err := parseTelemostInput(roomInput)
	if err != nil {
		return fmt.Errorf("parse telemost input: %w", err)
	}

	if name == "" {
		name = fmt.Sprintf("Bot%d", botID)
	}

	connInfo, err := telemostGetConnection(ctx, roomURL, name)
	if err != nil {
		return fmt.Errorf("get connection: %w", err)
	}

	log.Printf("[telemost] connecting to media server: %s", connInfo.MediaServerURL)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
	}
	wsConn, _, err := dialer.DialContext(ctx, connInfo.MediaServerURL, nil)
	if err != nil {
		return fmt.Errorf("connect media server: %w", err)
	}
	defer wsConn.Close()

	// Send hello message using Goolom protocol (from olcrtc reference)
	helloUID := uuid.New().String()
	helloMsg := map[string]interface{}{
		"uid": helloUID,
		"hello": map[string]interface{}{
			"participantMeta": map[string]interface{}{
				"name":        name,
				"role":        "SPEAKER",
				"description": "",
				"sendAudio":   true,
				"sendVideo":   false,
			},
			"participantAttributes": map[string]interface{}{
				"name":        name,
				"role":        "SPEAKER",
				"description": "",
			},
			"sendAudio":         true,
			"sendVideo":         false,
			"sendSharing":       false,
			"participantId":     connInfo.PeerID,
			"roomId":            connInfo.RoomID,
			"serviceName":       "telemost",
			"credentials":       connInfo.Credentials,
			"capabilitiesOffer": goolomCapabilitiesOffer(),
			"sdkInfo": map[string]interface{}{
				"implementation": "browser",
				"version":        "5.27.0",
				"userAgent":      "Mozilla/5.0 (X11; Linux x86_64; rv:149.0) Gecko/20100101 Firefox/149.0",
				"hwConcurrency":  4,
			},
			"sdkInitializationId":    uuid.New().String(),
			"disablePublisher":       false,
			"disableSubscriber":      false,
			"disableSubscriberAudio": true,
		},
	}
	if err := wsConn.WriteJSON(helloMsg); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}
	log.Printf("[telemost] hello sent, uid=%s", helloUID)

	// Telemost uses PlanB SDP semantics — pion/webrtc defaults to UnifiedPlan.
	// Must create PeerConnections with PlanB semantics to match the server's SDP offers.
	telemostICEServers := []webrtc.ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	}

	// Create subscriber PeerConnection (PlanB)
	subPC, err := CreatePlanBPeerConnection(telemostICEServers)
	if err != nil {
		return fmt.Errorf("create subscriber pc: %w", err)
	}
	defer subPC.Close()

	// Add recvonly audio transceiver to subscriber PC
	if _, err := subPC.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio,
		webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		return fmt.Errorf("add subscriber audio transceiver: %w", err)
	}

	// Create publisher PeerConnection (PlanB) with sendonly audio track
	pubPC, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers:   telemostICEServers,
		SDPSemantics: webrtc.SDPSemanticsPlanB,
	})
	if err != nil {
		return fmt.Errorf("create publisher pc: %w", err)
	}
	defer pubPC.Close()

	pubTrack, err := CreateOpusTrack()
	if err != nil {
		return fmt.Errorf("create opus track: %w", err)
	}
	if _, err := pubPC.AddTrack(pubTrack); err != nil {
		return fmt.Errorf("add publisher track: %w", err)
	}

	// Process signaling messages
	msgChan := make(chan map[string]interface{}, 32)
	errChan := make(chan error, 1)

	go func() {
		for {
			_, msgBytes, err := wsConn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			var msg map[string]interface{}
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				continue
			}
			msgChan <- msg
		}
	}()

	publisherOffered := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			return fmt.Errorf("ws read: %w", err)
		case msg := <-msgChan:
			// Handle ack/ping/pong
			if _, ok := msg["ack"]; ok {
				continue
			}
			if _, ok := msg["pong"]; ok {
				continue
			}
			if _, ok := msg["serverHello"]; ok {
				log.Printf("[telemost] serverHello received")
				// Send ack for serverHello
				if uid, _ := msg["uid"].(string); uid != "" {
					wsConn.WriteJSON(map[string]interface{}{
						"uid": uid,
						"ack": map[string]interface{}{"status": map[string]interface{}{"code": "OK"}},
					})
				}
				continue
			}
			if _, ok := msg["ping"]; ok {
				if uid, _ := msg["uid"].(string); uid != "" {
					wsConn.WriteJSON(map[string]interface{}{"uid": uid, "pong": map[string]interface{}{}})
				}
				continue
			}

			msgType := ""
			if _, ok := msg["subscriberSdpOffer"]; ok {
				msgType = "subscriberSdpOffer"
			} else if _, ok := msg["publisherSdpAnswer"]; ok {
				msgType = "publisherSdpAnswer"
			} else if _, ok := msg["webrtcIceCandidate"]; ok {
				msgType = "webrtcIceCandidate"
			}

			log.Printf("[telemost] msg type=%s", msgType)

			switch msgType {
			case "subscriberSdpOffer":
				offer := msg["subscriberSdpOffer"].(map[string]interface{})
				sdp, _ := offer["sdp"].(string)
				msgUID, _ := msg["uid"].(string)

				log.Printf("[telemost] subscriber offer received, SDP length=%d", len(sdp))

				if err := subPC.SetRemoteDescription(webrtc.SessionDescription{
					Type: webrtc.SDPTypeOffer,
					SDP:  sdp,
				}); err != nil {
					return fmt.Errorf("set subscriber remote desc: %w", err)
				}

				answer, err := subPC.CreateAnswer(nil)
				if err != nil {
					return fmt.Errorf("create subscriber answer: %w", err)
				}

				gatherComplete := webrtc.GatheringCompletePromise(subPC)
				if err := subPC.SetLocalDescription(answer); err != nil {
					return fmt.Errorf("set subscriber local desc: %w", err)
				}

				log.Printf("[telemost] subscriber answer created, gathering ICE...")

				select {
				case <-gatherComplete:
				case <-time.After(10 * time.Second):
					return fmt.Errorf("subscriber ice gathering timeout")
				}

				log.Printf("[telemost] subscriber ICE gathering complete, sending answer")

				answerUID := uuid.New().String()
				answerMsg := map[string]interface{}{
					"uid": answerUID,
					"subscriberSdpAnswer": map[string]interface{}{
						"sdp":    subPC.LocalDescription().SDP,
						"peerId": connInfo.PeerID,
					},
				}
				if err := wsConn.WriteJSON(answerMsg); err != nil {
					return fmt.Errorf("send subscriber answer: %w", err)
				}

				// Send ack for the offer
				if msgUID != "" {
					wsConn.WriteJSON(map[string]interface{}{
						"uid": msgUID,
						"ack": map[string]interface{}{"status": map[string]interface{}{"code": "OK"}},
					})
				}

				// Now offer publisher
				if !publisherOffered {
					pubOffer, err := pubPC.CreateOffer(nil)
					if err != nil {
						return fmt.Errorf("create publisher offer: %w", err)
					}

					gatherPub := webrtc.GatheringCompletePromise(pubPC)
					if err := pubPC.SetLocalDescription(pubOffer); err != nil {
						return fmt.Errorf("set publisher local desc: %w", err)
					}

					log.Printf("[telemost] publisher offer created, gathering ICE...")

					select {
					case <-gatherPub:
					case <-time.After(10 * time.Second):
						return fmt.Errorf("publisher ice gathering timeout")
					}

					log.Printf("[telemost] publisher ICE gathering complete, sending offer")

					pubUID := uuid.New().String()
					pubOfferMsg := map[string]interface{}{
						"uid": pubUID,
						"publisherSdpOffer": map[string]interface{}{
							"sdp":    pubPC.LocalDescription().SDP,
							"peerId": connInfo.PeerID,
						},
					}
					if err := wsConn.WriteJSON(pubOfferMsg); err != nil {
						return fmt.Errorf("send publisher offer: %w", err)
					}
					publisherOffered = true
					log.Printf("[telemost] publisher offer sent")
				}

			case "publisherSdpAnswer":
				answer := msg["publisherSdpAnswer"].(map[string]interface{})
				sdp, _ := answer["sdp"].(string)

				if err := pubPC.SetRemoteDescription(webrtc.SessionDescription{
					Type: webrtc.SDPTypeAnswer,
					SDP:  sdp,
				}); err != nil {
					return fmt.Errorf("set publisher remote desc: %w", err)
				}

				// Send ack
				if msgUID, _ := msg["uid"].(string); msgUID != "" {
					wsConn.WriteJSON(map[string]interface{}{
						"uid": msgUID,
						"ack": map[string]interface{}{"status": map[string]interface{}{"code": "OK"}},
					})
				}

				log.Printf("[telemost] publisher SDP answer set, starting audio stream")
				if onStatus != nil {
					onStatus(StatusActive, "")
				}
				return StreamOpusFrames(ctx, pubTrack, opusFrames, loop)

			case "webrtcIceCandidate":
				cand := msg["webrtcIceCandidate"].(map[string]interface{})
				candidate, _ := cand["candidate"].(string)
				log.Printf("[telemost] ICE candidate received: %d chars", len(candidate))
				if candidate != "" {
					if err := subPC.AddICECandidate(webrtc.ICECandidateInit{Candidate: candidate}); err != nil {
						_ = pubPC.AddICECandidate(webrtc.ICECandidateInit{Candidate: candidate})
					}
				}
			}
		}
	}
}

type telemostConnInfo struct {
	PeerID         string
	RoomID         string
	Credentials    string
	MediaServerURL string
}

func parseTelemostInput(input string) (string, error) {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return input, nil
	}
	if isNumeric(input) {
		return fmt.Sprintf("https://telemost.yandex.ru/j/%s", input), nil
	}
	return "", fmt.Errorf("invalid telemost input: %s", input)
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// telemostGetConnection fetches connection info from Yandex Telemost API.
// Based on olcrtc/internal/auth/telemost/api.go — does NOT require OAuth token.
// The Telemost API works without authentication for public conferences.
func telemostGetConnection(ctx context.Context, roomURL, displayName string) (*telemostConnInfo, error) {
	u := fmt.Sprintf(
		"https://cloud-api.yandex.ru/telemost_front/v2/telemost/conferences/%s/connection",
		url.QueryEscape(roomURL),
	)

	log.Printf("[telemost] requesting connection info: %s", u)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}

	// Query parameters required by Telemost API (matching olcrtc reference)
	q := req.URL.Query()
	q.Add("next_gen_media_platform_allowed", "true")
	q.Add("display_name", displayName)
	q.Add("waiting_room_supported", "true")
	req.URL.RawQuery = q.Encode()

	// Headers matching olcrtc/internal/auth/telemost/api.go
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:149.0) Gecko/20100101 Firefox/149.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client-Instance-Id", uuid.New().String())
	req.Header.Set("X-Telemost-Client-Version", "187.1.0")
	req.Header.Set("Idempotency-Key", uuid.New().String())
	req.Header.Set("Origin", "https://telemost.yandex.ru")
	req.Header.Set("Referer", "https://telemost.yandex.ru/")

	client := newRetryHTTPClient(30 * time.Second)

	log.Printf("[telemost] GET %s headers=%v", req.URL.String(), req.Header)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[telemost] connection response status=%d body=%s", resp.StatusCode, string(respBody))

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("telemost api status %d: %s", resp.StatusCode, string(respBody))
	}

	// Response format from olcrtc:
	// {"room_id":"...","peer_id":"...","credentials":"...","client_configuration":{"media_server_url":"..."}}
	var result struct {
		RoomID       string `json:"room_id"`
		PeerID       string `json:"peer_id"`
		Credentials  string `json:"credentials"`
		ClientConfig struct {
			MediaServerURL string `json:"media_server_url"`
		} `json:"client_configuration"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if result.PeerID == "" || result.ClientConfig.MediaServerURL == "" {
		return nil, fmt.Errorf("incomplete connection info: %+v", result)
	}

	return &telemostConnInfo{
		PeerID:         result.PeerID,
		RoomID:         result.RoomID,
		Credentials:    result.Credentials,
		MediaServerURL: result.ClientConfig.MediaServerURL,
	}, nil
}

// newRetryHTTPClient creates an HTTP client with retry on transient DNS/dial errors,
// matching olcrtc/internal/protect/protect.go.
func newRetryHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &retryTransport{
			base: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          10,
				IdleConnTimeout:       30 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
			},
		},
	}
}

type retryTransport struct {
	base http.RoundTripper
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	const maxRetries = 3
	var resp *http.Response
	var err error
	for i := range maxRetries {
		if i > 0 {
			time.Sleep(time.Duration(i) * 500 * time.Millisecond)
		}
		resp, err = t.base.RoundTrip(req)
		if err == nil || !isRetriableError(err) {
			if err != nil {
				return resp, fmt.Errorf("round trip: %w", err)
			}
			return resp, nil
		}
	}
	return resp, fmt.Errorf("round trip after %d retries: %w", maxRetries, err)
}

func isRetriableError(err error) bool {
	if err == nil {
		return false
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Timeout() || strings.Contains(opErr.Error(), "connection refused")
	}
	s := err.Error()
	return strings.Contains(s, "no such host") ||
		strings.Contains(s, "connection reset") ||
		strings.Contains(s, "i/o timeout")
}

// goolomCapabilitiesOffer returns the capabilities offer object for the Goolom hello message.
// Based on olcrtc/internal/engine/goolom/session.go — includes all required fields.
func goolomCapabilitiesOffer() map[string]interface{} {
	return map[string]interface{}{
		"audio": map[string]interface{}{
			"codecs": []map[string]interface{}{
				{
					"mime":            "audio/opus",
					"clockRate":       48000,
					"channels":        1,
					"maxplaybackrate": 48000,
					"stereo":          0,
					"sprop-stereo":    0,
					"useinbandfec":    1,
					"usedtx":          0,
				},
			},
			"extensions": []map[string]interface{}{
				{"uri": "urn:ietf:params:rtp-hdrext:ssrc-audio-level"},
				{"uri": "urn:ietf:params:rtp-hdrext:csrc-audio-level"},
				{"uri": "urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id"},
			},
		},
		"video": map[string]interface{}{
			"codecs":     []interface{}{},
			"extensions": []interface{}{},
		},
		"offerAnswerMode":        "offer",
		"initialSubscriberOffer": true,
		"slotsMode":              "single",
		"simulcastMode":          "rtx",
		"selfVadStatus":          "off",
		"dataChannelSharing":     false,
	}
}
