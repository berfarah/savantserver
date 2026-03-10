package main

import (
	"encoding/json"
	"testing"
)

// --- Discovery payload tests ---

func TestBuildDiscoveryPayload_Dimmable(t *testing.T) {
	entity := &LightEntity{
		UniqueID:    "savant_load_005_0",
		Name:        "Lights",
		RoomName:    "Den",
		RoomSlug:    "den",
		LoadSlug:    "lights",
		IsDimmable:  true,
		DeviceModel: "ECHO Adaptive phase",
	}

	data, err := buildDiscoveryPayload(entity, "savant", "homeassistant")
	if err != nil {
		t.Fatalf("buildDiscoveryPayload() error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	// Check top-level fields
	assertStr(t, got, "schema", "json")
	assertStr(t, got, "name", "Lights")
	assertStr(t, got, "unique_id", "savant_load_005_0")
	assertStr(t, got, "object_id", "savant_load_005_0")
	assertStr(t, got, "state_topic", "savant/den/lights/light/state")
	assertStr(t, got, "command_topic", "savant/den/lights/light/set")
	assertStr(t, got, "availability_topic", "savant/status")
	assertStr(t, got, "payload_available", "online")
	assertStr(t, got, "payload_not_available", "offline")

	// Dimmable fields must be present
	if b, ok := got["brightness"].(bool); !ok || !b {
		t.Error("expected brightness: true for dimmable light")
	}
	if s, ok := got["brightness_scale"].(float64); !ok || s != 100 {
		t.Errorf("expected brightness_scale: 100, got %v", got["brightness_scale"])
	}

	// Check device block
	dev, ok := got["device"].(map[string]interface{})
	if !ok {
		t.Fatal("expected device block to be an object")
	}
	assertStr(t, dev, "name", "Den Lights")
	assertStr(t, dev, "manufacturer", "Savant")
	assertStr(t, dev, "model", "ECHO Adaptive phase")
	assertStr(t, dev, "suggested_area", "Den")

	ids, ok := dev["identifiers"].([]interface{})
	if !ok || len(ids) != 1 || ids[0] != "savant_load_005_0" {
		t.Errorf("expected identifiers: [savant_load_005_0], got %v", dev["identifiers"])
	}
}

func TestBuildDiscoveryPayload_Switch(t *testing.T) {
	entity := &LightEntity{
		UniqueID:    "savant_load_003_2",
		Name:        "Sconces",
		RoomName:    "Master Bath",
		RoomSlug:    "master_bath",
		LoadSlug:    "sconces",
		IsDimmable:  false,
		DeviceModel: "Relay module",
	}

	data, err := buildDiscoveryPayload(entity, "savant", "homeassistant")
	if err != nil {
		t.Fatalf("buildDiscoveryPayload() error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	// brightness and brightness_scale must be absent for non-dimmable
	if _, ok := got["brightness"]; ok {
		t.Error("brightness field should be absent for switch")
	}
	if _, ok := got["brightness_scale"]; ok {
		t.Error("brightness_scale field should be absent for switch")
	}

	assertStr(t, got, "name", "Sconces")
	assertStr(t, got, "state_topic", "savant/master_bath/sconces/light/state")
	assertStr(t, got, "command_topic", "savant/master_bath/sconces/light/set")
}

// --- State payload tests ---

func TestBuildStatePayload_DimmableOn(t *testing.T) {
	entity := &LightEntity{IsDimmable: true}
	state := LightState{On: true, Brightness: 75}

	data, err := buildStatePayload(entity, state)
	if err != nil {
		t.Fatalf("buildStatePayload() error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	assertStr(t, got, "state", "ON")
	if b, ok := got["brightness"].(float64); !ok || b != 75 {
		t.Errorf("expected brightness: 75, got %v", got["brightness"])
	}
}

func TestBuildStatePayload_DimmableOff(t *testing.T) {
	entity := &LightEntity{IsDimmable: true}
	state := LightState{On: false, Brightness: 0}

	data, err := buildStatePayload(entity, state)
	if err != nil {
		t.Fatalf("buildStatePayload() error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	assertStr(t, got, "state", "OFF")
	if b, ok := got["brightness"].(float64); !ok || b != 0 {
		t.Errorf("expected brightness: 0, got %v", got["brightness"])
	}
}

func TestBuildStatePayload_SwitchOn(t *testing.T) {
	entity := &LightEntity{IsDimmable: false}
	state := LightState{On: true}

	data, err := buildStatePayload(entity, state)
	if err != nil {
		t.Fatalf("buildStatePayload() error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	assertStr(t, got, "state", "ON")
	if _, ok := got["brightness"]; ok {
		t.Error("brightness should be absent for switch")
	}
}

func TestBuildStatePayload_SwitchOff(t *testing.T) {
	entity := &LightEntity{IsDimmable: false}
	state := LightState{On: false}

	data, err := buildStatePayload(entity, state)
	if err != nil {
		t.Fatalf("buildStatePayload() error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	assertStr(t, got, "state", "OFF")
	if _, ok := got["brightness"]; ok {
		t.Error("brightness should be absent for switch")
	}
}

func TestBuildStatePayload_DimmableOnBrightnessUnknown(t *testing.T) {
	entity := &LightEntity{IsDimmable: true}
	state := LightState{On: true, Brightness: -1}

	data, err := buildStatePayload(entity, state)
	if err != nil {
		t.Fatalf("buildStatePayload() error: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("JSON unmarshal error: %v", err)
	}

	assertStr(t, got, "state", "ON")
	// brightness should be omitted when unknown (-1)
	if _, ok := got["brightness"]; ok {
		t.Error("brightness should be absent when unknown (-1)")
	}
}

// --- Command parsing tests ---

func TestCommandParsing_StateOnOnly(t *testing.T) {
	var cmd LightCommand
	if err := json.Unmarshal([]byte(`{"state":"ON"}`), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.State == nil || *cmd.State != "ON" {
		t.Errorf("expected state ON, got %v", cmd.State)
	}
	if cmd.Brightness != nil {
		t.Error("expected brightness to be nil")
	}
}

func TestCommandParsing_StateOnWithBrightness(t *testing.T) {
	var cmd LightCommand
	if err := json.Unmarshal([]byte(`{"state":"ON","brightness":50}`), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.State == nil || *cmd.State != "ON" {
		t.Errorf("expected state ON, got %v", cmd.State)
	}
	if cmd.Brightness == nil || *cmd.Brightness != 50 {
		t.Errorf("expected brightness 50, got %v", cmd.Brightness)
	}
}

func TestCommandParsing_StateOff(t *testing.T) {
	var cmd LightCommand
	if err := json.Unmarshal([]byte(`{"state":"OFF"}`), &cmd); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if cmd.State == nil || *cmd.State != "OFF" {
		t.Errorf("expected state OFF, got %v", cmd.State)
	}
	if cmd.Brightness != nil {
		t.Error("expected brightness to be nil")
	}
}

// --- Topic parsing tests ---

func TestParseCommandTopic_Valid(t *testing.T) {
	tests := []struct {
		topic    string
		prefix   string
		wantID   string
	}{
		{"savant/den/lights/light/set", "savant", "den/lights"},
		{"savant/master_bath/sconces/light/set", "savant", "master_bath/sconces"},
		{"custom/prefix/evelyns_room/night_stand/light/set", "custom/prefix", "evelyns_room/night_stand"},
	}
	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			got, err := parseCommandTopic(tt.topic, tt.prefix)
			if err != nil {
				t.Fatalf("parseCommandTopic(%q, %q) error: %v", tt.topic, tt.prefix, err)
			}
			if got != tt.wantID {
				t.Errorf("parseCommandTopic(%q, %q) = %q, want %q", tt.topic, tt.prefix, got, tt.wantID)
			}
		})
	}
}

func TestParseCommandTopic_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		topic  string
		prefix string
	}{
		{"wrong prefix", "other/den/lights/light/set", "savant"},
		{"too few parts", "savant/den/light/set", "savant"},
		{"too many parts", "savant/den/lights/extra/light/set", "savant"},
		{"wrong suffix", "savant/den/lights/light/get", "savant"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCommandTopic(tt.topic, tt.prefix)
			if err == nil {
				t.Errorf("parseCommandTopic(%q, %q) expected error, got nil", tt.topic, tt.prefix)
			}
		})
	}
}

// --- Test helpers ---

func assertStr(t *testing.T, m map[string]interface{}, key, want string) {
	t.Helper()
	got, ok := m[key].(string)
	if !ok {
		t.Errorf("key %q: expected string, got %T (%v)", key, m[key], m[key])
		return
	}
	if got != want {
		t.Errorf("key %q: got %q, want %q", key, got, want)
	}
}
