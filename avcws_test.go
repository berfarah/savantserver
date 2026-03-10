package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// testAVCServer creates an httptest server that speaks the avc WebSocket protocol.
// It returns the server and a channel that receives messages sent by the client.
func testAVCServer(t *testing.T) (*httptest.Server, chan avcMessage) {
	t.Helper()
	received := make(chan avcMessage, 32)

	upgrader := websocket.Upgrader{
		Subprotocols: []string{"savant_protocol"},
		CheckOrigin:  func(r *http.Request) bool { return true },
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Read the devicePresent handshake
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read handshake: %v", err)
			return
		}
		var handshake avcMessage
		if err := json.Unmarshal(data, &handshake); err != nil {
			t.Errorf("parse handshake: %v", err)
			return
		}
		if handshake.URI != "session/devicePresent" {
			t.Errorf("expected devicePresent, got %q", handshake.URI)
			return
		}
		received <- handshake

		// Respond with deviceRecognized
		resp := avcMessage{
			URI:      "session/deviceRecognized",
			Messages: []map[string]any{{"status": "ok"}},
		}
		if err := conn.WriteJSON(resp); err != nil {
			t.Errorf("write deviceRecognized: %v", err)
			return
		}

		// Forward all subsequent messages to the received channel
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return // connection closed
			}
			var msg avcMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			received <- msg
		}
	}))

	return server, received
}

func wsURL(server *httptest.Server) (string, int) {
	// Convert http://127.0.0.1:PORT to host and port
	addr := server.Listener.Addr().String()
	parts := strings.Split(addr, ":")
	host := parts[0]
	port := 0
	fmt.Sscanf(parts[1], "%d", &port)
	return host, port
}

func TestHandshakeMessage(t *testing.T) {
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

	data, err := json.Marshal(handshake)
	if err != nil {
		t.Fatalf("marshal handshake: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["URI"] != "session/devicePresent" {
		t.Errorf("URI = %v, want session/devicePresent", parsed["URI"])
	}

	msgs := parsed["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages length = %d, want 1", len(msgs))
	}

	msg := msgs[0].(map[string]any)
	if msg["protocolVersion"] != "0.1" {
		t.Errorf("protocolVersion = %v, want 0.1", msg["protocolVersion"])
	}

	device := msg["device"].(map[string]any)
	if device["name"] != "savantserver" {
		t.Errorf("device.name = %v, want savantserver", device["name"])
	}
	if device["app"] != "savantserver" {
		t.Errorf("device.app = %v, want savantserver", device["app"])
	}
}

func TestSubscribeMessage(t *testing.T) {
	msg := avcMessage{
		URI: "state/register",
		Messages: []map[string]any{
			{"state": "module"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"URI":"state/register","messages":[{"state":"module"}]}`
	if string(data) != want {
		t.Errorf("Subscribe JSON =\n  %s\nwant:\n  %s", string(data), want)
	}
}

func TestSetLoadMessage(t *testing.T) {
	msg := avcMessage{
		URI: "state/set",
		Messages: []map[string]any{
			{
				"state": fmt.Sprintf("load.%x", 0x50000),
				"value": "75%.0",
			},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// json.Marshal sorts keys alphabetically within objects
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed["URI"] != "state/set" {
		t.Errorf("URI = %v, want state/set", parsed["URI"])
	}

	msgs := parsed["messages"].([]any)
	m := msgs[0].(map[string]any)
	if m["state"] != "load.50000" {
		t.Errorf("state = %v, want load.50000", m["state"])
	}
	if m["value"] != "75%.0" {
		t.Errorf("value = %v, want 75%%.0", m["value"])
	}
}

func TestSimulateButtonPressMessages(t *testing.T) {
	addrStr := fmt.Sprintf("switch.%x", 0x50002)

	pressMsg := avcMessage{
		URI: "state/set",
		Messages: []map[string]any{
			{"state": addrStr, "value": "press"},
		},
	}
	releaseMsg := avcMessage{
		URI: "state/set",
		Messages: []map[string]any{
			{"state": addrStr, "value": "release"},
		},
	}

	pressData, _ := json.Marshal(pressMsg)
	releaseData, _ := json.Marshal(releaseMsg)

	var pp, rp map[string]any
	json.Unmarshal(pressData, &pp)
	json.Unmarshal(releaseData, &rp)

	pm := pp["messages"].([]any)[0].(map[string]any)
	if pm["state"] != "switch.50002" {
		t.Errorf("press state = %v, want switch.50002", pm["state"])
	}
	if pm["value"] != "press" {
		t.Errorf("press value = %v, want press", pm["value"])
	}

	rm := rp["messages"].([]any)[0].(map[string]any)
	if rm["state"] != "switch.50002" {
		t.Errorf("release state = %v, want switch.50002", rm["state"])
	}
	if rm["value"] != "release" {
		t.Errorf("release value = %v, want release", rm["value"])
	}
}

// --- Integration tests using a test WebSocket server ---

func TestConnectHandshake(t *testing.T) {
	server, received := testAVCServer(t)
	defer server.Close()

	host, port := wsURL(server)
	client := NewAVCClient(host, port)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Verify the handshake message was received by the server
	select {
	case msg := <-received:
		if msg.URI != "session/devicePresent" {
			t.Errorf("handshake URI = %q, want session/devicePresent", msg.URI)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for handshake")
	}
}

func TestSubscribeIntegration(t *testing.T) {
	server, received := testAVCServer(t)
	defer server.Close()

	host, port := wsURL(server)
	client := NewAVCClient(host, port)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Drain the handshake message
	<-received

	if err := client.Subscribe("module"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	select {
	case msg := <-received:
		if msg.URI != "state/register" {
			t.Errorf("URI = %q, want state/register", msg.URI)
		}
		if len(msg.Messages) != 1 {
			t.Fatalf("messages length = %d, want 1", len(msg.Messages))
		}
		if msg.Messages[0]["state"] != "module" {
			t.Errorf("state = %v, want module", msg.Messages[0]["state"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for subscribe message")
	}
}

func TestSetLoadIntegration(t *testing.T) {
	server, received := testAVCServer(t)
	defer server.Close()

	host, port := wsURL(server)
	client := NewAVCClient(host, port)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	<-received // drain handshake

	if err := client.SetLoad(0x50000, "75%.0"); err != nil {
		t.Fatalf("SetLoad: %v", err)
	}

	select {
	case msg := <-received:
		if msg.URI != "state/set" {
			t.Errorf("URI = %q, want state/set", msg.URI)
		}
		if msg.Messages[0]["state"] != "load.50000" {
			t.Errorf("state = %v, want load.50000", msg.Messages[0]["state"])
		}
		if msg.Messages[0]["value"] != "75%.0" {
			t.Errorf("value = %v, want 75%%.0", msg.Messages[0]["value"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SetLoad message")
	}
}

func TestReadLoopStateUpdate(t *testing.T) {
	// Create a custom server that sends a state update after handshake
	upgrader := websocket.Upgrader{
		Subprotocols: []string{"savant_protocol"},
		CheckOrigin:  func(r *http.Request) bool { return true },
	}

	var serverConn *websocket.Conn
	var connReady sync.WaitGroup
	connReady.Add(1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		serverConn, err = upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}

		// Read and respond to handshake
		_, _, err = serverConn.ReadMessage()
		if err != nil {
			t.Errorf("read handshake: %v", err)
			return
		}
		resp := avcMessage{
			URI:      "session/deviceRecognized",
			Messages: []map[string]any{{"status": "ok"}},
		}
		serverConn.WriteJSON(resp)
		connReady.Done()

		// Keep the handler alive so the connection stays open
		select {}
	}))
	defer server.Close()

	host, port := wsURL(server)
	client := NewAVCClient(host, port)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Wait for server to be ready
	connReady.Wait()

	// Start ReadLoop in a goroutine
	updates := make(chan AVCStateUpdate, 8)
	go client.ReadLoop(ctx, func(update AVCStateUpdate) {
		updates <- update
	})

	// Send a state update from the server
	stateUpdate := avcMessage{
		URI: "state/update",
		Messages: []map[string]any{
			{"state": "module.50000", "value": "75,100,X,-1"},
		},
	}
	serverConn.WriteJSON(stateUpdate)

	// Verify the update is received
	select {
	case update := <-updates:
		if update.State != "module.50000" {
			t.Errorf("State = %q, want module.50000", update.State)
		}
		if update.Value != "75,100,X,-1" {
			t.Errorf("Value = %q, want 75,100,X,-1", update.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for state update")
	}

	// Verify non-state/update messages are ignored
	otherMsg := avcMessage{
		URI:      "state/register/response",
		Messages: []map[string]any{{"status": "ok"}},
	}
	serverConn.WriteJSON(otherMsg)

	// Send another state update to verify the loop is still working
	stateUpdate2 := avcMessage{
		URI: "state/update",
		Messages: []map[string]any{
			{"state": "scene.4", "value": "1"},
		},
	}
	serverConn.WriteJSON(stateUpdate2)

	select {
	case update := <-updates:
		if update.State != "scene.4" {
			t.Errorf("State = %q, want scene.4", update.State)
		}
		if update.Value != "1" {
			t.Errorf("Value = %q, want 1", update.Value)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for second state update")
	}
}
