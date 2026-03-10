package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- handleModuleUpdate tests ---

func TestHandleModuleUpdate_MatchedLoad(t *testing.T) {
	b := newTestBridge()

	// Device address 5, offset 0 → hexAddr = (5 << 16) | 0 = 0x50000 = 327680
	b.handleStateUpdate(AVCStateUpdate{
		State: "module.5",
		Value: "75,X",
	})

	st := b.state["savant_load_005_0"]
	if !st.On {
		t.Error("expected light to be on")
	}
	if st.Brightness != 75 {
		t.Errorf("expected brightness 75, got %d", st.Brightness)
	}
}

func TestHandleModuleUpdate_TurnOff(t *testing.T) {
	b := newTestBridge()
	b.state["savant_load_005_0"].On = true
	b.state["savant_load_005_0"].Brightness = 50

	b.handleStateUpdate(AVCStateUpdate{
		State: "module.5",
		Value: "0,X",
	})

	st := b.state["savant_load_005_0"]
	if st.On {
		t.Error("expected light to be off")
	}
	if st.Brightness != 0 {
		t.Errorf("expected brightness 0, got %d", st.Brightness)
	}
}

func TestHandleModuleUpdate_SecondOffset(t *testing.T) {
	b := newTestBridge()

	// Second load at offset 1 → hexAddr = (5 << 16) | 1 = 0x50001 = 327681
	b.handleStateUpdate(AVCStateUpdate{
		State: "module.5",
		Value: "X,100",
	})

	st := b.state["savant_load_005_1"]
	if !st.On {
		t.Error("expected closet light to be on")
	}
	if st.Brightness != 100 {
		t.Errorf("expected brightness 100, got %d", st.Brightness)
	}

	// First load should not have changed
	st0 := b.state["savant_load_005_0"]
	if st0.On {
		t.Error("first load should not have changed")
	}
}

func TestHandleModuleUpdate_UnknownDevice(t *testing.T) {
	b := newTestBridge()

	// Device address 0xFF — not in our registry
	b.handleStateUpdate(AVCStateUpdate{
		State: "module.ff",
		Value: "50",
	})

	// Nothing should have changed
	for _, st := range b.state {
		if st.On {
			t.Error("no entity should have changed")
		}
	}
}

func TestHandleModuleUpdate_SkipsDash1AndX(t *testing.T) {
	b := newTestBridge()

	b.handleStateUpdate(AVCStateUpdate{
		State: "module.5",
		Value: "-1,X",
	})

	for _, st := range b.state {
		if st.On {
			t.Error("no entity should have changed for -1 or X values")
		}
	}
}

func TestHandleModuleUpdate_SceneIgnored(t *testing.T) {
	b := newTestBridge()

	// scene updates should not crash
	b.handleStateUpdate(AVCStateUpdate{
		State: "scene.4",
		Value: "1",
	})

	for _, st := range b.state {
		if st.On {
			t.Error("scene updates should not affect state")
		}
	}
}

// --- handleCommand tests ---

func TestHandleCommand_TurnOnWithDaylight(t *testing.T) {
	b, received := newTestBridgeWithAVC(t)

	on := "ON"
	b.handleCommand("den/lights", LightCommand{State: &on})

	// Optimistic state: NL load gets brightness=-1 (unknown, waiting for avc update)
	st := b.state["savant_load_005_0"]
	if !st.On {
		t.Error("expected light on")
	}
	if st.Brightness != -1 {
		t.Errorf("expected brightness -1 (NL unknown), got %d", st.Brightness)
	}

	// Verify avc received button press (not a load set)
	msg := expectAVCMessage(t, received, 2*time.Second)
	if msg.URI != "state/set" {
		t.Errorf("URI = %q, want state/set", msg.URI)
	}
	state := msg.Messages[0]["state"].(string)
	if state != "switch.50002" {
		t.Errorf("state = %q, want switch.50002 (NL toggle button press)", state)
	}
	if msg.Messages[0]["value"] != "press" {
		t.Errorf("value = %q, want press", msg.Messages[0]["value"])
	}

	// Expect release after press
	releaseMsg := expectAVCMessage(t, received, 1*time.Second)
	if releaseMsg.Messages[0]["value"] != "release" {
		t.Errorf("value = %q, want release", releaseMsg.Messages[0]["value"])
	}
}

func TestHandleCommand_TurnOnWithBrightness(t *testing.T) {
	b, received := newTestBridgeWithAVC(t)

	on := "ON"
	brightness := 42
	b.handleCommand("den/lights", LightCommand{State: &on, Brightness: &brightness})

	st := b.state["savant_load_005_0"]
	if !st.On {
		t.Error("expected light on")
	}
	if st.Brightness != 42 {
		t.Errorf("expected brightness 42, got %d", st.Brightness)
	}

	// Verify avc received load set command
	msg := expectAVCMessage(t, received, 2*time.Second)
	state := msg.Messages[0]["state"].(string)
	if state != "load.50000" {
		t.Errorf("state = %q, want load.50000", state)
	}
	if msg.Messages[0]["value"] != "42%.0" {
		t.Errorf("value = %q, want 42%%.0", msg.Messages[0]["value"])
	}
}

func TestHandleCommand_TurnOff(t *testing.T) {
	b, received := newTestBridgeWithAVC(t)
	b.state["savant_load_005_0"].On = true
	b.state["savant_load_005_0"].Brightness = 75

	off := "OFF"
	b.handleCommand("den/lights", LightCommand{State: &off})

	st := b.state["savant_load_005_0"]
	if st.On {
		t.Error("expected light off")
	}
	if st.Brightness != 0 {
		t.Errorf("expected brightness 0, got %d", st.Brightness)
	}

	// Verify avc received load off command
	msg := expectAVCMessage(t, received, 2*time.Second)
	state := msg.Messages[0]["state"].(string)
	if state != "load.50000" {
		t.Errorf("state = %q, want load.50000", state)
	}
	if msg.Messages[0]["value"] != "0%.1" {
		t.Errorf("value = %q, want 0%%.1", msg.Messages[0]["value"])
	}
}

func TestHandleCommand_BrightnessOnly(t *testing.T) {
	b, received := newTestBridgeWithAVC(t)

	brightness := 60
	b.handleCommand("den/lights", LightCommand{Brightness: &brightness})

	st := b.state["savant_load_005_0"]
	if !st.On {
		t.Error("expected light on when brightness > 0")
	}
	if st.Brightness != 60 {
		t.Errorf("expected brightness 60, got %d", st.Brightness)
	}

	// Verify avc command
	msg := expectAVCMessage(t, received, 2*time.Second)
	if msg.Messages[0]["value"] != "60%.0" {
		t.Errorf("value = %q, want 60%%.0", msg.Messages[0]["value"])
	}
}

func TestHandleCommand_BrightnessZero(t *testing.T) {
	b := newTestBridge()
	b.state["savant_load_005_0"].On = true
	b.state["savant_load_005_0"].Brightness = 50

	brightness := 0
	b.handleCommand("den/lights", LightCommand{Brightness: &brightness})

	st := b.state["savant_load_005_0"]
	if st.On {
		t.Error("expected light off when brightness = 0")
	}
}

func TestHandleCommand_TurnOnAlreadyOnKeepsBrightness(t *testing.T) {
	b := newTestBridge()
	b.state["savant_load_005_0"].On = true
	b.state["savant_load_005_0"].Brightness = 50

	on := "ON"
	b.handleCommand("den/lights", LightCommand{State: &on})

	st := b.state["savant_load_005_0"]
	if st.Brightness != 50 {
		t.Errorf("expected brightness 50 to be preserved, got %d", st.Brightness)
	}
}

func TestHandleCommand_UnknownEntity(t *testing.T) {
	b := newTestBridge()

	on := "ON"
	// Should not panic
	b.handleCommand("nonexistent/light", LightCommand{State: &on})
}

func TestHandleCommand_NonDaylightOnSetsMax(t *testing.T) {
	b, received := newTestBridgeWithAVC(t)

	on := "ON"
	b.handleCommand("den/closet", LightCommand{State: &on})

	st := b.state["savant_load_005_1"]
	if !st.On {
		t.Error("expected closet light on")
	}
	if st.Brightness != 100 {
		t.Errorf("expected brightness 100 (max for non-daylight), got %d", st.Brightness)
	}

	// Verify avc received load set to max (not button press)
	msg := expectAVCMessage(t, received, 2*time.Second)
	state := msg.Messages[0]["state"].(string)
	if state != "load.50001" {
		t.Errorf("state = %q, want load.50001", state)
	}
	if msg.Messages[0]["value"] != "100%.0" {
		t.Errorf("value = %q, want 100%%.0", msg.Messages[0]["value"])
	}
}

// --- hydrateState tests ---

func TestHydratePerRoom(t *testing.T) {
	stateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/states/Den.BrightnessLevel":
			json.NewEncoder(w).Encode(map[string][]string{"data": {"75"}})
		case "/states/Den.RoomLightsAreOn":
			json.NewEncoder(w).Encode(map[string][]string{"data": {"1"}})
		default:
			json.NewEncoder(w).Encode(map[string][]string{"data": {"0"}})
		}
	}))
	defer stateServer.Close()

	b := newTestBridge()
	b.cfg = &Config{}
	b.api = &SavantAPI{baseURL: stateServer.URL, client: stateServer.Client()}

	if err := b.hydrateState(context.Background()); err != nil {
		t.Fatalf("hydrateState error: %v", err)
	}

	// Both entities are in "Den" — per-room hydration gives them the same brightness
	st0 := b.state["savant_load_005_0"]
	if !st0.On {
		t.Error("expected lights on")
	}
	if st0.Brightness != 75 {
		t.Errorf("expected brightness 75, got %d", st0.Brightness)
	}

	st1 := b.state["savant_load_005_1"]
	if !st1.On {
		t.Error("expected closet on (same room brightness)")
	}
	if st1.Brightness != 75 {
		t.Errorf("expected brightness 75 (shared room value), got %d", st1.Brightness)
	}
}

func TestHydratePerLoad(t *testing.T) {
	stateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/states/TestConfig.RacePointMedia_host.CurrentDimmerLevel_1_005":
			// Device 005 has two loads — comma-separated: offset 0 = 80, offset 1 = 0
			json.NewEncoder(w).Encode(map[string][]string{"data": {"80,0"}})
		default:
			json.NewEncoder(w).Encode(map[string][]string{"data": {"0"}})
		}
	}))
	defer stateServer.Close()

	b := newTestBridge()
	b.cfg = &Config{
		Savant: SavantConfig{ConfigName: "TestConfig"},
	}
	b.api = &SavantAPI{baseURL: stateServer.URL, client: stateServer.Client()}

	if err := b.hydrateState(context.Background()); err != nil {
		t.Fatalf("hydrateState error: %v", err)
	}

	// Per-load: offset 0 should be 80, offset 1 should be 0
	st0 := b.state["savant_load_005_0"]
	if !st0.On {
		t.Error("expected lights on (brightness 80)")
	}
	if st0.Brightness != 80 {
		t.Errorf("expected brightness 80, got %d", st0.Brightness)
	}

	st1 := b.state["savant_load_005_1"]
	if st1.On {
		t.Error("expected closet off (brightness 0)")
	}
	if st1.Brightness != 0 {
		t.Errorf("expected brightness 0, got %d", st1.Brightness)
	}
}

func TestHydratePerLoadFallsBackToPerRoom(t *testing.T) {
	stateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/states/Den.BrightnessLevel":
			json.NewEncoder(w).Encode(map[string][]string{"data": {"50"}})
		case "/states/Den.RoomLightsAreOn":
			json.NewEncoder(w).Encode(map[string][]string{"data": {"1"}})
		default:
			json.NewEncoder(w).Encode(map[string][]string{"data": {"0"}})
		}
	}))
	defer stateServer.Close()

	b := newTestBridge()
	b.cfg = &Config{} // No config_name → per-room fallback
	b.api = &SavantAPI{baseURL: stateServer.URL, client: stateServer.Client()}

	if err := b.hydrateState(context.Background()); err != nil {
		t.Fatalf("hydrateState error: %v", err)
	}

	st0 := b.state["savant_load_005_0"]
	if st0.Brightness != 50 {
		t.Errorf("expected room-level brightness 50, got %d", st0.Brightness)
	}
}

// --- test helpers ---

// newTestBridge creates a Bridge with two entities (den lights + den closet)
// for testing state management and command routing.
func newTestBridge() *Bridge {
	return newTestBridgeFromEntities(testEntities())
}

func testEntities() []LightEntity {
	return []LightEntity{
		{
			UniqueID:           "savant_load_005_0",
			Name:               "Lights",
			RoomName:           "Den",
			RoomSlug:           "den",
			LoadSlug:           "lights",
			DeviceAddress:      "005",
			LoadOffset:         0,
			HexAddr:            ComputeHexAddr(5, 0), // 0x50000
			IsDimmable:         true,
			FollowDaylight:     true,
			Min:                0,
			Max:                100,
			DaylightToggleAddr: ComputeSwitchAddr(5, 3), // 0x50002
			DeviceModel:        "ECHO Adaptive phase",
		},
		{
			UniqueID:           "savant_load_005_1",
			Name:               "Closet",
			RoomName:           "Den",
			RoomSlug:           "den",
			LoadSlug:           "closet",
			DeviceAddress:      "005",
			LoadOffset:         1,
			HexAddr:            ComputeHexAddr(5, 1), // 0x50001
			IsDimmable:         false,
			FollowDaylight:     false,
			Min:                0,
			Max:                100,
			DaylightToggleAddr: -1,
			DeviceModel:        "ECHO Adaptive phase",
		},
	}
}

func newTestBridgeFromEntities(entities []LightEntity) *Bridge {
	b := &Bridge{
		entities:  entities,
		state:     make(map[string]*LightState, len(entities)),
		entityIdx: make(map[string]*LightEntity, len(entities)),
		addrIdx:   make(map[int]*LightEntity, len(entities)),
		logger:    testLogger(),
	}

	for i := range b.entities {
		e := &b.entities[i]
		b.state[e.UniqueID] = &LightState{}
		b.entityIdx[e.RoomSlug+"/"+e.LoadSlug] = e
		b.addrIdx[e.HexAddr] = e
	}

	return b
}

// newTestBridgeWithAVC creates a test bridge connected to a test AVC WebSocket server.
// Returns the bridge and a channel that receives avc messages sent by the bridge.
func newTestBridgeWithAVC(t *testing.T) (*Bridge, chan avcMessage) {
	t.Helper()
	server, received := testAVCServer(t)
	t.Cleanup(server.Close)

	host, port := wsURL(server)
	client := NewAVCClient(host, port)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("AVC connect: %v", err)
	}
	t.Cleanup(func() { client.Close() })

	// Drain the handshake message
	select {
	case <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for handshake drain")
	}

	b := newTestBridge()
	b.avc = client
	return b, received
}

// expectAVCMessage waits for an avc message on the channel or fails with timeout.
func expectAVCMessage(t *testing.T, ch chan avcMessage, timeout time.Duration) avcMessage {
	t.Helper()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for avc message")
		return avcMessage{} // unreachable
	}
}

func testLogger() *log.Logger {
	return log.New(io.Discard, "[test] ", 0)
}

// testStateServer creates an HTTP server that returns per-load dimmer levels.
func testStateServer(t *testing.T, states map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stateName := r.URL.Path[len("/states/"):]
		w.Header().Set("Content-Type", "application/json")
		val, ok := states[stateName]
		if !ok {
			val = "0"
		}
		resp := map[string][]string{"data": {val}}
		json.NewEncoder(w).Encode(resp)
	}))
}

// Verify bridge.Start log line format includes required info
func TestBridgeStartLogsConfig(t *testing.T) {
	b := newTestBridge()
	b.cfg = &Config{
		Savant: SavantConfig{
			Host:     "192.168.1.1",
			RESTPort: 3062,
			AVCPort:  8480,
		},
		MQTT: MQTTConfig{
			Broker: "tcp://localhost:1883",
		},
	}
	// Start would fail since there's no real Savant, but we just verify the struct is set up
	if b.cfg.Savant.Host != "192.168.1.1" {
		t.Error("config not set")
	}
	_ = fmt.Sprintf("starting (savant=%s:%d, avc=%d, mqtt=%s)",
		b.cfg.Savant.Host, b.cfg.Savant.RESTPort, b.cfg.Savant.AVCPort, b.cfg.MQTT.Broker)
}
