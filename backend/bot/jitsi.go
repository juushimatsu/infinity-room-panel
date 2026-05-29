package bot

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/pion/webrtc/v4"
)

// RunJitsiBot connects a bot to a Jitsi Meet room via XMPP-over-WebSocket
// and streams Opus audio frames.
// Based on the j library (github.com/zarazaex69/j) reference implementation.
func RunJitsiBot(ctx context.Context, botID int, name, roomURL string, opusFrames [][]byte, loop bool, onStatus StatusCallback) error {
	host, room, err := parseJitsiURL(roomURL)
	if err != nil {
		return fmt.Errorf("parse jitsi url: %w", err)
	}

	// Fetch MUC and XMPP domains from /config.js (like j-master does)
	mucDomain, xmppDomain := fetchJitsiConfig(host)
	if mucDomain == "" {
		mucDomain = "conference." + host
	}
	if xmppDomain == "" {
		xmppDomain = host
	}

	// Jitsi XMPP WebSocket URL — based on j-master Dial()
	wsURL := fmt.Sprintf("wss://%s/xmpp-websocket?room=%s", host, room)

	log.Printf("[jitsi] connecting to %s (muc=%s, xmpp=%s)", wsURL, mucDomain, xmppDomain)

	// WebSocket dial using coder/websocket — matching j-master Dial()
	// coder/websocket handles subprotocol negotiation correctly (gorilla/websocket does not)
	//
	// Provide a custom HTTPClient with HTTP/2 and TLS support.
	// Some Jitsi servers (especially behind reverse proxies/CDN) require HTTP/2 ALPN
	// negotiation before accepting WebSocket upgrade. The default http.Client used
	// by coder/websocket.Dial doesn't enable HTTP/2 by default.
	wsHTTPClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // Self-hosted Jitsi may use self-signed certs
			ForceAttemptHTTP2:   true,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	wsOpts := &websocket.DialOptions{
		Subprotocols:    []string{"xmpp"},
		CompressionMode: websocket.CompressionContextTakeover,
		HTTPClient:      wsHTTPClient,
		HTTPHeader: http.Header{
			"Accept":          {"*/*"},
			"Accept-Language": {"en-US,en;q=0.9"},
			"Cache-Control":   {"no-cache"},
			"Origin":          {"https://" + host},
			"Pragma":          {"no-cache"},
			"Sec-Fetch-Dest":  {"empty"},
			"Sec-Fetch-Mode":  {"websocket"},
			"Sec-Fetch-Site":  {"same-origin"},
			"User-Agent":      {"Mozilla/5.0 (X11; Linux x86_64; rv:150.0) Gecko/20100101 Firefox/150.0"},
		},
	}

	wsConn, resp, err := websocket.Dial(ctx, wsURL, wsOpts)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			log.Printf("[jitsi] primary handshake failed: %v, status=%d, body=%s", err, resp.StatusCode, string(body))
		} else {
			log.Printf("[jitsi] primary handshake failed (no response): %v", err)
		}

		// Try alternate WebSocket path (some servers don't accept ?room= param)
		altURL := fmt.Sprintf("wss://%s/xmpp-websocket", host)
		log.Printf("[jitsi] trying alternate URL: %s", altURL)

		wsConn2, resp2, err2 := websocket.Dial(ctx, altURL, wsOpts)
		if err2 != nil {
			if resp2 != nil {
				body2, _ := io.ReadAll(io.LimitReader(resp2.Body, 4096))
				log.Printf("[jitsi] alternate handshake failed: %v, status=%d, body=%s", err2, resp2.StatusCode, string(body2))
			} else {
				log.Printf("[jitsi] alternate handshake failed (no response): %v", err2)
			}
			return fmt.Errorf("connect xmpp ws: %w (alt: %w)", err, err2)
		}
		wsConn = wsConn2
	}
	defer wsConn.Close(websocket.StatusInternalError, "")

	wsConn.SetReadLimit(1 << 20) // 1MB — matching j-master

	log.Printf("[jitsi] connected to XMPP WebSocket")

	mucNick := fmt.Sprintf("bot%d", botID)
	jid, focusJID, err := jitsiXMPPHandshake(ctx, wsConn, host, room, mucNick, name, mucDomain, xmppDomain)
	if err != nil {
		return fmt.Errorf("xmpp handshake: %w", err)
	}

	log.Printf("[jitsi] XMPP handshake complete, jid=%s focus=%s", jid, focusJID)

	// Jitsi Jingle flow: Jicofo (focus) initiates the session, NOT the participant.
	// After joining MUC, Jicofo sends session-initiate to each participant.
	// The participant must wait for it, then respond with session-accept.
	// This is the opposite of what was previously implemented.
	log.Printf("[jitsi] waiting for session-initiate from focus...")

	// Wait for session-initiate from focus (Jicofo)
	jingleMsg, err := jitsiWaitForJingleInitiate(ctx, wsConn)
	if err != nil {
		return fmt.Errorf("wait session-initiate: %w", err)
	}

	log.Printf("[jitsi] received session-initiate from focus")

	// Extract SDP offer and session info from Jingle stanza
	remoteSDP, jingleSID, initiatorJID, focusRoomJID, err := parseJingleInitiate(jingleMsg)
	if err != nil {
		return fmt.Errorf("parse session-initiate: %w", err)
	}

	log.Printf("[jitsi] jingle sid=%s initiator=%s from=%s", jingleSID, initiatorJID, focusRoomJID)

	// Create PeerConnection with sendonly audio track
	pc, track, err := CreateSendOnlyPeerConnection()
	if err != nil {
		return fmt.Errorf("create pc: %w", err)
	}
	defer pc.Close()

	// Set the remote description (offer from Jicofo)
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  remoteSDP,
	}); err != nil {
		return fmt.Errorf("set remote desc: %w", err)
	}

	// Create answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return fmt.Errorf("create answer: %w", err)
	}

	// Wait for ICE gathering to complete
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	if err := pc.SetLocalDescription(answer); err != nil {
		return fmt.Errorf("set local desc: %w", err)
	}

	select {
	case <-gatherComplete:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("ice gathering timeout")
	case <-ctx.Done():
		return ctx.Err()
	}

	// Send session-accept to focus with our answer SDP
	localSDP := pc.LocalDescription().SDP
	acceptID := fmt.Sprintf("sa%d", botID)
	sessionAccept := fmt.Sprintf(
		`<iq type="set" to="%s" id="%s" xmlns="jabber:client"><jingle xmlns="urn:xmpp:jingle:1" action="session-accept" initiator="%s" responder="%s" sid="%s"><content creator="initiator" name="audio"><description xmlns="urn:xmpp:jingle:apps:rtp:1" media="audio"><payload-type id="111" name="opus" clockrate="48000" channels="1"/></description><transport xmlns="urn:xmpp:jingle:transports:ice-udp:1"/></content></jingle></iq>`,
		focusRoomJID, acceptID, initiatorJID, jid, jingleSID,
	)

	if err := jitsiSend(ctx, wsConn, sessionAccept); err != nil {
		return fmt.Errorf("send session-accept: %w", err)
	}

	log.Printf("[jitsi] session-accept sent, SDP answer length=%d", len(localSDP))

	// Now handle incoming transport-info (ICE candidates) while audio streams
	errCh := make(chan error, 1)
	go func() {
		// Read and process transport-info messages until context is cancelled
		for {
			msg, err := jitsiRead(ctx, wsConn)
			if err != nil {
				errCh <- err
				return
			}
			if strings.Contains(msg, "transport-info") {
				// ICE candidate — extract and add to PC
				jitsiHandleTransportInfo(msg, pc)
			}
		}
	}()

	log.Printf("[jitsi] session established, starting audio stream")
	if onStatus != nil {
		onStatus(StatusActive, "")
	}
	audioErr := StreamOpusFrames(ctx, track, opusFrames, loop)

	select {
	case err := <-errCh:
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("xmpp read: %w", err)
	default:
		return audioErr
	}
}

func parseJitsiURL(rawURL string) (host, room string, err error) {
	rawURL = strings.TrimSpace(rawURL)

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid url: %w", err)
	}
	host = u.Host
	room = strings.TrimPrefix(u.Path, "/")
	if room == "" {
		return "", "", fmt.Errorf("empty room name in url")
	}
	return host, room, nil
}

// fetchJitsiConfig downloads /config.js from the Jitsi host and extracts
// the MUC and XMPP domains. Based on j-master fetchConfig().
func fetchJitsiConfig(host string) (mucDomain, xmppDomain string) {
	mucDomain = "conference." + host
	xmppDomain = host

	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	resp, err := client.Get("https://" + host + "/config.js")
	if err != nil {
		log.Printf("[jitsi] fetch config.js failed: %v", err)
		return
	}
	defer resp.Body.Close()

	buf := make([]byte, 64*1024)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	if v := jitsiExtractStringField(body, "domain"); v != "" {
		xmppDomain = v
	}
	if v := jitsiExtractStringField(body, "muc"); v != "" {
		mucDomain = v
	}

	log.Printf("[jitsi] config.js: xmppDomain=%s mucDomain=%s", xmppDomain, mucDomain)
	return
}

// jitsiExtractStringField finds `<key>: <expr>` or `<key> = <expr>` in JS source
// and returns the concatenation of all string literals in <expr>.
// Supports JS string concatenation like: 'conference.' + subdomain + 'host'
// Based on j-master extractStringField().
func jitsiExtractStringField(body, key string) string {
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		// Strip leading "config.hosts." / "config." prefixes for assignment style.
		t = strings.TrimPrefix(t, "config.hosts.")
		t = strings.TrimPrefix(t, "config.")
		if !strings.HasPrefix(t, key) {
			continue
		}
		rest := strings.TrimPrefix(t, key)
		// require ":", "=", or whitespace after key (avoid matching e.g. "domain2")
		if len(rest) == 0 || (rest[0] != ':' && rest[0] != '=' && rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		rest = strings.TrimLeft(rest, " \t:=")
		// strip trailing comment / semicolon / comma — only the expression matters
		if i := strings.IndexAny(rest, ";,/"); i >= 0 {
			rest = rest[:i]
		}
		v := jitsiJoinStringLiterals(rest)
		if v != "" {
			return v
		}
	}
	return ""
}

// jitsiJoinStringLiterals walks a JS expression and concatenates all single- or
// double-quoted string literals. Other tokens (identifiers, "+", whitespace)
// are ignored. Based on j-master joinStringLiterals().
func jitsiJoinStringLiterals(expr string) string {
	var out strings.Builder
	for i := 0; i < len(expr); i++ {
		c := expr[i]
		if c != '\'' && c != '"' {
			continue
		}
		end := strings.IndexByte(expr[i+1:], c)
		if end < 0 {
			break
		}
		out.WriteString(expr[i+1 : i+1+end])
		i += 1 + end
	}
	return out.String()
}

// Jitsi caps — matching j-master calculateJitsiCapsVersion().
// The caps version is a SHA-1 hash of sorted feature strings, base64-encoded.
// This is required for Jicofo to recognize the client as a valid Jitsi participant.
const jitsiCapsNode = "https://jitsi.org/jitsi-meet"

var jitsiMeetFeatures = []string{
	"http://jabber.org/protocol/caps",
	"http://jitsi.org/json-encoded-sources",
	"http://jitsi.org/receive-multiple-video-streams",
	"http://jitsi.org/remb",
	"http://jitsi.org/source-name",
	"http://jitsi.org/start-muted-room-metadata",
	"http://jitsi.org/tcc",
	"http://jitsi.org/visitors-1",
	"urn:ietf:rfc:4588",
	"urn:xmpp:jingle:1",
	"urn:xmpp:jingle:apps:dtls:0",
	"urn:xmpp:jingle:apps:rtp:1",
	"urn:xmpp:jingle:apps:rtp:audio",
	"urn:xmpp:jingle:apps:rtp:video",
	"urn:xmpp:jingle:transports:dtls-sctp:1",
	"urn:xmpp:jingle:transports:ice-udp:1",
}

var jitsiCapsVersion = calculateJitsiCapsVersion()

func calculateJitsiCapsVersion() string {
	features := append([]string(nil), jitsiMeetFeatures...)
	sort.Strings(features)
	var s strings.Builder
	for _, feature := range features {
		s.WriteString(feature)
		s.WriteByte('<')
	}
	sum := sha1.Sum([]byte(s.String()))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// jitsiSend writes a message to the XMPP WebSocket connection.
func jitsiSend(ctx context.Context, ws *websocket.Conn, msg string) error {
	log.Printf("[jitsi-xmpp] send: %s", truncate(msg, 200))
	return ws.Write(ctx, websocket.MessageText, []byte(msg))
}

// jitsiRead reads one message from the XMPP WebSocket connection.
func jitsiRead(ctx context.Context, ws *websocket.Conn) (string, error) {
	_, msg, err := ws.Read(ctx)
	if err != nil {
		return "", fmt.Errorf("read xmpp: %w", err)
	}
	msgStr := string(msg)
	log.Printf("[jitsi-xmpp] recv: %s", truncate(msgStr, 300))
	return msgStr, nil
}

// jitsiReadUntil reads from WebSocket until a message containing the substring is found.
// Returns the matching message. Based on j-master readUntil.
func jitsiReadUntil(ctx context.Context, ws *websocket.Conn, substr string) error {
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for {
		msg, err := jitsiRead(readCtx, ws)
		if err != nil {
			return err
		}
		if strings.Contains(msg, substr) {
			return nil
		}
		if strings.Contains(msg, "stream:error") || strings.Contains(msg, "<failure") {
			return fmt.Errorf("server error: %s", msg)
		}
	}
}

// jitsiReadUntilReturn reads from WebSocket until a message containing the substring is found.
// Returns the matching message. Based on j-master readUntilReturn.
func jitsiReadUntilReturn(ctx context.Context, ws *websocket.Conn, substr string) (string, error) {
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for {
		msg, err := jitsiRead(readCtx, ws)
		if err != nil {
			return "", err
		}
		if strings.Contains(msg, substr) {
			return msg, nil
		}
		if strings.Contains(msg, "stream:error") || strings.Contains(msg, "<failure") {
			return "", fmt.Errorf("server error: %s", msg)
		}
	}
}

// jitsiXMPPHandshake performs the full XMPP auth flow based on j-master auth().
// Flow: open stream → check ANONYMOUS → SASL ANONYMOUS → reopen stream → bind → session →
// stream management → discover services → allocate focus → join MUC
func jitsiXMPPHandshake(ctx context.Context, ws *websocket.Conn, host, room, nick, displayName, mucDomain, xmppDomain string) (jid, focusJID string, err error) {
	// 1. Open XMPP stream (to xmppDomain — based on j-master)
	openStream := fmt.Sprintf(`<open to="%s" version="1.0" xmlns="urn:ietf:params:xml:ns:xmpp-framing"/>`, xmppDomain)
	if err := jitsiSend(ctx, ws, openStream); err != nil {
		return "", "", err
	}

	// Wait for stream features — check for ANONYMOUS mechanism (based on j-master auth)
	initialFeatures, err := jitsiReadUntilReturn(ctx, ws, "features")
	if err != nil {
		return "", "", fmt.Errorf("initial features: %w", err)
	}
	if !strings.Contains(initialFeatures, "<mechanism>ANONYMOUS</mechanism>") {
		return "", "", fmt.Errorf("server does not advertise anonymous XMPP login")
	}

	// 2. SASL ANONYMOUS authentication
	if err := jitsiSend(ctx, ws, `<auth mechanism="ANONYMOUS" xmlns="urn:ietf:params:xml:ns:xmpp-sasl"/>`); err != nil {
		return "", "", err
	}
	if err := jitsiReadUntil(ctx, ws, "success"); err != nil {
		return "", "", fmt.Errorf("sasl anonymous: %w", err)
	}

	// 3. Reopen stream after SASL (based on j-master auth())
	if err := jitsiSend(ctx, ws, openStream); err != nil {
		return "", "", err
	}
	if err := jitsiReadUntil(ctx, ws, "features"); err != nil {
		return "", "", fmt.Errorf("post-auth features: %w", err)
	}

	// 4. Bind resource
	if err := jitsiSend(ctx, ws, `<iq type="set" id="bind_1" xmlns="jabber:client"><bind xmlns="urn:ietf:params:xml:ns:xmpp-bind"/></iq>`); err != nil {
		return "", "", err
	}
	bindResp, err := jitsiReadUntilReturn(ctx, ws, "<jid>")
	if err != nil {
		return "", "", fmt.Errorf("bind: %w", err)
	}

	// Extract JID from bind response (based on j-master extractJID)
	if idx := strings.Index(bindResp, "<jid>"); idx != -1 {
		start := idx + 5
		end := strings.Index(bindResp[start:], "</jid>")
		if end != -1 {
			jid = bindResp[start : start+end]
		}
	}
	if jid == "" {
		return "", "", fmt.Errorf("bind failed: %s", bindResp)
	}

	// Derive nick from JID (matching j-master: first 8 chars of JID local part)
	parts := strings.Split(jid, "@")
	if len(parts) > 0 && len(parts[0]) >= 8 {
		nick = parts[0][:8]
	}

	// 5. Session
	if err := jitsiSend(ctx, ws, `<iq type="set" id="sess_1" xmlns="jabber:client"><session xmlns="urn:ietf:params:xml:ns:xmpp-session"/></iq>`); err != nil {
		return "", "", err
	}
	if err := jitsiReadUntil(ctx, ws, "sess_1"); err != nil {
		return "", "", fmt.Errorf("session: %w", err)
	}

	// 6. Enable stream management (based on j-master auth())
	if err := jitsiSend(ctx, ws, `<enable resume="true" xmlns="urn:xmpp:sm:3"/>`); err != nil {
		return "", "", err
	}
	if err := jitsiReadUntil(ctx, ws, "enabled"); err != nil {
		log.Printf("[jitsi] stream management not supported, continuing")
		// Not all servers support SM, continue anyway
	}

	// 7. Discover services (based on j-master DiscoverServices)
	// Send disco#info to the XMPP domain, not the host
	if err := jitsiSend(ctx, ws, fmt.Sprintf(
		`<iq type="get" to="%s" id="disco1" xmlns="jabber:client"><query xmlns="http://jabber.org/protocol/disco#info"/></iq>`, xmppDomain,
	)); err != nil {
		return "", "", err
	}
	// Wait for disco response (don't need to parse it for basic functionality)
	if _, err := jitsiReadUntilReturn(ctx, ws, "disco1"); err != nil {
		log.Printf("[jitsi] disco response: %v, continuing", err)
	}

	// 8. Allocate focus (based on j-master AllocateFocus)
	// CRITICAL: target is focus.{xmppDomain}, NOT focus.{host}
	// CRITICAL: xmlns is http://jitsi.org/protocol/focus, NOT http://jit.si/jit-conference
	// CRITICAL: properties must match what Jicofo expects
	focusJID = fmt.Sprintf("focus.%s", xmppDomain)
	roomJID := fmt.Sprintf("%s@%s", room, mucDomain)
	allocateIQ := fmt.Sprintf(
		`<iq to="%s" type="set" id="focus_1" xmlns="jabber:client"><conference room="%s" machine-uid="%s" xmlns="http://jitsi.org/protocol/focus"><property name="rtcstatsEnabled" value="false"/><property name="visitors-version" value="1"/></conference></iq>`,
		focusJID, roomJID, nick,
	)
	if err := jitsiSend(ctx, ws, allocateIQ); err != nil {
		return "", "", err
	}

	// Wait for focus allocation response (based on j-master AllocateFocus)
	// Must look for "conference" + "ready" in the response, not just the IQ id
	focusAllocated := false
	readCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	for {
		msg, err := jitsiRead(readCtx, ws)
		if err != nil {
			cancel()
			return "", "", fmt.Errorf("allocate focus: %w", err)
		}
		if strings.Contains(msg, "conference") && strings.Contains(msg, "ready") {
			focusAllocated = true
			break
		}
		// Check for IQ error on focus_1
		if strings.Contains(msg, "error") && strings.Contains(msg, "focus_1") {
			cancel()
			return "", "", fmt.Errorf("focus allocation failed: %s", msg)
		}
	}
	cancel()

	if !focusAllocated {
		return "", "", fmt.Errorf("focus allocation timeout")
	}
	log.Printf("[jitsi] focus allocated successfully")

	// 9. Join MUC room (based on j-master JoinMUC)
	mucJID := fmt.Sprintf("%s@%s/%s", room, mucDomain, nick)
	statsID := "abp-j"
	// Use runes (not bytes) for stats-id — Russian names are multi-byte UTF-8.
	// j-master uses displayName[:3] which works on bytes in Go, but for
	// multi-byte UTF-8 we need rune slicing to avoid invalid UTF-8 sequences
	// that would make the XML not-well-formed.
	runes := []rune(displayName)
	if len(runes) >= 3 {
		statsID = string(runes[:3]) + "-j"
	} else if len(runes) > 0 {
		statsID = string(runes[:1]) + "-j"
	}

	presence := fmt.Sprintf(
		`<presence to="%s" xmlns="jabber:client"><x xmlns="http://jabber.org/protocol/muc"/><stats-id>%s</stats-id><c hash="sha-1" node="%s" ver="%s" xmlns="http://jabber.org/protocol/caps"/><SourceInfo>{}</SourceInfo><jitsi_participant_codecList>vp8,h264,av1,vp9</jitsi_participant_codecList><nick xmlns="http://jabber.org/protocol/nick">%s</nick></presence>`,
		mucJID, statsID, jitsiCapsNode, jitsiCapsVersion, xmlEscape(displayName),
	)
	if err := jitsiSend(ctx, ws, presence); err != nil {
		return "", "", err
	}

	// Wait for self-presence (status code 110) — based on j-master JoinMUC
	joinCtx, joinCancel := context.WithTimeout(ctx, 15*time.Second)
	for {
		msg, err := jitsiRead(joinCtx, ws)
		if err != nil {
			joinCancel()
			return "", "", fmt.Errorf("muc join: %w", err)
		}
		if strings.Contains(msg, `status code="110"`) || strings.Contains(msg, `code='110'`) {
			log.Printf("[jitsi] MUC joined successfully")
			break
		}
	}
	joinCancel()

	// 10. Send room info (based on j-master SendRoomInfo)
	roomInfoIQ := fmt.Sprintf(
		`<iq type="get" to="%s@%s" id="roominfo1" xmlns="jabber:client"><query xmlns="http://jabber.org/protocol/disco#info"/></iq>`,
		room, mucDomain,
	)
	if err := jitsiSend(ctx, ws, roomInfoIQ); err != nil {
		log.Printf("[jitsi] send room info: %v", err)
	}

	return jid, focusJID, nil
}

// jitsiWaitForJingleInitiate waits for a Jingle session-initiate stanza from focus (Jicofo).
// After joining MUC, Jicofo sends session-initiate to each participant.
// The participant must wait for this instead of initiating itself.
func jitsiWaitForJingleInitiate(ctx context.Context, ws *websocket.Conn) (string, error) {
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		msg, err := jitsiRead(waitCtx, ws)
		if err != nil {
			return "", fmt.Errorf("wait jingle initiate: %w", err)
		}

		if strings.Contains(msg, "session-initiate") {
			return msg, nil
		}

		// Skip non-jingle messages (presence, messages, etc.)
		if strings.Contains(msg, "transport-info") {
			log.Printf("[jitsi] early transport-info (before session), skipping")
			continue
		}
	}
}

// parseJingleInitiate extracts SDP offer, SID, initiator JID, and sender JID
// from a Jingle session-initiate IQ stanza.
func parseJingleInitiate(iq string) (sdp, sid, initiatorJID, fromJID string, err error) {
	// Extract 'from' attribute (the sender, e.g. room@conference.domain/focus)
	fromJID = extractXMLAttr(iq, "from")

	// Extract 'initiator' attribute from <jingle> element
	initiatorJID = extractXMLAttr(iq, "initiator")

	// Extract 'sid' attribute from <jingle> element
	sid = extractXMLAttr(iq, "sid")

	if sid == "" {
		return "", "", "", "", fmt.Errorf("no sid in jingle stanza")
	}

	// Extract SDP — Jicofo sends it as base64 in <parameter name="sdp-...">
	sdp = extractSDPFromJingle(iq)
	if sdp == "" {
		return "", "", "", "", fmt.Errorf("no sdp in session-initiate")
	}

	return sdp, sid, initiatorJID, fromJID, nil
}

// jitsiHandleTransportInfo processes a transport-info IQ and adds ICE candidates
// to the PeerConnection.
func jitsiHandleTransportInfo(msg string, pc *webrtc.PeerConnection) {
	// Extract candidate from <candidate> element
	candidate := ""
	if idx := strings.Index(msg, "<candidate"); idx != -1 {
		// Extract the content between tags or from attr
		if cIdx := strings.Index(msg[idx:], "name=\"sdp-0\""); cIdx != -1 {
			// sdp-0 candidate parameter
			if valIdx := strings.Index(msg[idx+cIdx:], "value=\""); valIdx != -1 {
				start := idx + cIdx + valIdx + 7
				end := strings.Index(msg[start:], "\"")
				if end != -1 {
					candidate = msg[start : start+end]
				}
			}
		}
	}

	if candidate != "" {
		if err := pc.AddICECandidate(webrtc.ICECandidateInit{Candidate: candidate}); err != nil {
			log.Printf("[jitsi] add ICE candidate error: %v", err)
		} else {
			log.Printf("[jitsi] added ICE candidate")
		}
	}
}

// extractXMLAttr extracts an XML attribute value from a string.
func extractXMLAttr(xml, attr string) string {
	// Look for attr='value' or attr="value"
	for _, quote := range []string{"'", "\""} {
		pattern := attr + "=" + quote
		if idx := strings.Index(xml, pattern); idx != -1 {
			start := idx + len(pattern)
			end := strings.Index(xml[start:], quote)
			if end != -1 {
				return xml[start : start+end]
			}
		}
	}
	return ""
}

func extractSDPFromJingle(iq string) string {
	// Try to find base64-encoded SDP in <parameter name="sdp-...">
	if idx := strings.Index(iq, "name=\"sdp-"); idx != -1 {
		valIdx := strings.Index(iq[idx:], "value=\"")
		if valIdx != -1 {
			start := idx + valIdx + 7
			end := strings.Index(iq[start:], "\"")
			if end != -1 {
				return iq[start : start+end]
			}
		}
	}

	if idx := strings.Index(iq, "<sdp>"); idx != -1 {
		start := idx + 5
		end := strings.Index(iq[start:], "</sdp>")
		if end != -1 {
			return iq[start : start+end]
		}
	}

	// Try raw SDP in the stanza
	if strings.Contains(iq, "v=0") && strings.Contains(iq, "o=") {
		start := strings.Index(iq, "v=0")
		end := strings.LastIndex(iq, "</")
		if end > start {
			return iq[start:end]
		}
	}

	return ""
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// xmlEscape escapes special characters for use in XML.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
