package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AVCClient connects to the Savant avc lighting controller WebSocket (default
// port 8480). This is the hardware control plane — it handles dimmer levels,
// button press simulation, and real-time state subscriptions for lighting
// modules and scenes. Uses the "savant_protocol" WebSocket subprotocol.
// See docs/04-avc-websocket.md for the full protocol reference.

// avcMessage is the JSON envelope used by all avc WebSocket messages.
type avcMessage struct {
	URI      string           `json:"URI"`
	Messages []map[string]any `json:"messages"`
}

// AVCClient is a WebSocket client for the Savant avc endpoint (port 8480).
type AVCClient struct {
	url    string
	conn   *websocket.Conn
	mu     sync.Mutex // protects writes to conn
	logger *log.Logger
}

// NewAVCClient creates a new AVCClient targeting the given host and port.
func NewAVCClient(host string, port int) *AVCClient {
	return &AVCClient{
		url:    fmt.Sprintf("ws://%s:%d", host, port),
		logger: log.New(os.Stderr, "[avc] ", log.LstdFlags),
	}
}

// Connect dials the avc WebSocket endpoint and performs the session handshake.
func (a *AVCClient) Connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		Subprotocols: []string{"savant_protocol"},
	}

	conn, _, err := dialer.DialContext(ctx, a.url, nil)
	if err != nil {
		return fmt.Errorf("avc dial: %w", err)
	}
	a.conn = conn

	// Send devicePresent handshake
	handshake := avcMessage{
		URI: "session/devicePresent",
		Messages: []map[string]any{
			{
				"protocolVersion": "0.1",
				"device": map[string]any{
					"name":    "savantserver",
					"version": "1.0",
					"app":     "savantserver",
					"ip":      "127.0.0.1",
					"model":   "server",
				},
			},
		},
	}
	if err := a.sendJSON(handshake); err != nil {
		a.conn.Close()
		return fmt.Errorf("avc handshake send: %w", err)
	}

	// Wait for deviceRecognized response
	for {
		_, data, err := a.conn.ReadMessage()
		if err != nil {
			a.conn.Close()
			return fmt.Errorf("avc handshake read: %w", err)
		}

		var msg avcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			a.logger.Printf("ignoring malformed handshake message: %v", err)
			continue
		}

		if msg.URI == "session/deviceRecognized" {
			a.logger.Printf("connected and recognized")
			return nil
		}
	}
}

// Close closes the WebSocket connection.
func (a *AVCClient) Close() error {
	if a.conn != nil {
		return a.conn.Close()
	}
	return nil
}

// Subscribe registers for state updates in the given categories (e.g. "module", "scene").
func (a *AVCClient) Subscribe(categories ...string) error {
	for _, cat := range categories {
		msg := avcMessage{
			URI: "state/register",
			Messages: []map[string]any{
				{"state": cat},
			},
		}
		if err := a.sendJSON(msg); err != nil {
			return fmt.Errorf("avc subscribe %q: %w", cat, err)
		}
	}
	return nil
}

// SetLoad sends a load state change command.
// hexAddr is the full hex address (e.g. 0x50000), value is the target (e.g. "75%.0").
func (a *AVCClient) SetLoad(hexAddr int, value string) error {
	msg := avcMessage{
		URI: "state/set",
		Messages: []map[string]any{
			{
				"state": fmt.Sprintf("load.%x", hexAddr),
				"value": value,
			},
		},
	}
	return a.sendJSON(msg)
}

// SimulateButtonPress sends a press then release for the given switch address.
func (a *AVCClient) SimulateButtonPress(switchAddr int) error {
	addrStr := fmt.Sprintf("switch.%x", switchAddr)

	pressMsg := avcMessage{
		URI: "state/set",
		Messages: []map[string]any{
			{"state": addrStr, "value": "press"},
		},
	}
	if err := a.sendJSON(pressMsg); err != nil {
		return fmt.Errorf("avc button press: %w", err)
	}

	time.Sleep(200 * time.Millisecond)

	releaseMsg := avcMessage{
		URI: "state/set",
		Messages: []map[string]any{
			{"state": addrStr, "value": "release"},
		},
	}
	if err := a.sendJSON(releaseMsg); err != nil {
		return fmt.Errorf("avc button release: %w", err)
	}

	return nil
}

// ReadLoop reads messages from the WebSocket and dispatches state updates to handler.
// It blocks until ctx is cancelled or the connection is closed.
func (a *AVCClient) ReadLoop(ctx context.Context, handler func(AVCStateUpdate)) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, data, err := a.conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return // context cancelled, expected shutdown
			}
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				a.logger.Printf("connection closed")
				return
			}
			a.logger.Printf("read error: %v", err)
			return
		}

		var msg avcMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			a.logger.Printf("ignoring malformed message: %v", err)
			continue
		}

		if msg.URI != "state/update" {
			continue
		}

		if len(msg.Messages) == 0 {
			a.logger.Printf("state/update with no messages")
			continue
		}

		stateVal, ok := msg.Messages[0]["state"]
		if !ok {
			a.logger.Printf("state/update missing 'state' field")
			continue
		}
		valueVal, ok := msg.Messages[0]["value"]
		if !ok {
			a.logger.Printf("state/update missing 'value' field")
			continue
		}

		state, _ := stateVal.(string)
		value, _ := valueVal.(string)

		handler(AVCStateUpdate{
			State: state,
			Value: value,
		})
	}
}

// sendJSON marshals and writes a JSON message, guarded by the write mutex.
func (a *AVCClient) sendJSON(msg avcMessage) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.conn.WriteJSON(msg)
}
