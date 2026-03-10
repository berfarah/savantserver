package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// --- REST API model types (match JSON from Savant REST API) ---

type Room struct {
	RoomID          string  `json:"roomID"`
	ConfigurationID string  `json:"configurationID"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	FollowDaylight  *bool   `json:"followDaylight"`
}

type Load struct {
	LoadID         string `json:"loadID"`
	DeviceID       string `json:"deviceID"`
	RoomID         string `json:"roomID"`
	Name           string `json:"name"`
	Offset         int    `json:"offset"`
	Type           int    `json:"type"` // 0 = dimmable, 1 = switch
	IsWired        bool   `json:"isWired"`
	Min            int    `json:"min"`
	Max            int    `json:"max"`
	TracksDevice   bool   `json:"tracksDevice"`
	FollowDaylight bool   `json:"followDaylight"`
}

type Device struct {
	DeviceID             string   `json:"deviceID"`
	RoomID               string   `json:"roomID"`
	Address              string   `json:"address"` // hex string, e.g. "005"
	Name                 string   `json:"name"`
	DeviceType           string   `json:"type"`
	BoardName            string   `json:"boardName"`
	HasWiredLoad         bool     `json:"hasWiredLoad"`
	HasEnergyMonitoring  bool     `json:"hasEnergyMonitoring"`
	Ambient              bool     `json:"ambient"`
	IsConnected          bool     `json:"isConnected"`
	RPMLightingDeviceName string  `json:"RPMLightingDeviceName"`
	Loads                []Load   `json:"loads,omitempty"`
	Buttons              []Button `json:"buttons,omitempty"`
}

type Button struct {
	ButtonID        string  `json:"buttonID"`
	DeviceID        string  `json:"deviceID"`
	LightingSceneID string  `json:"lightingSceneID"`
	RoomID          string  `json:"RoomID"` // note: uppercase R in JSON
	Label           string  `json:"label"`
	Scenes          string  `json:"scenes"`
	Index           int     `json:"index"`
	Function        string  `json:"function"`
	Command         string  `json:"command"`
	ToggleCommand   string  `json:"toggleCommand"`
	LEDBehavior     int     `json:"ledBehavior"`
}

// --- Bridge domain types ---

// LightEntity represents a single HA light entity mapped from a Savant load.
type LightEntity struct {
	UniqueID       string // e.g. "savant_load_005_0"
	Name           string // human-readable load name, e.g. "Lights"
	RoomName       string
	RoomSlug       string
	LoadSlug       string
	DeviceAddress  string // hex, e.g. "005"
	LoadOffset     int
	HexAddr        int    // computed: (deviceAddr << 16) | loadOffset
	IsDimmable     bool   // type 0 = true, type 1 = false
	FollowDaylight bool
	Min            int
	Max            int
	DaylightToggleAddr int // switch address for natural light toggle button, -1 if none
	DeviceModel    string // e.g. "ECHO Adaptive phase"
}

// StateTopic returns the MQTT state topic for this entity.
func (e *LightEntity) StateTopic(prefix string) string {
	return fmt.Sprintf("%s/%s/%s/light/state", prefix, e.RoomSlug, e.LoadSlug)
}

// CommandTopic returns the MQTT command topic for this entity.
func (e *LightEntity) CommandTopic(prefix string) string {
	return fmt.Sprintf("%s/%s/%s/light/set", prefix, e.RoomSlug, e.LoadSlug)
}

// DiscoveryTopic returns the HA MQTT Discovery config topic.
func (e *LightEntity) DiscoveryTopic(haPrefix string) string {
	return fmt.Sprintf("%s/light/%s/config", haPrefix, e.UniqueID)
}

// LightState represents the current state of a light entity.
type LightState struct {
	On         bool
	Brightness int // 0-100
}

// LightCommand represents a command received from HA via MQTT.
type LightCommand struct {
	State      *string `json:"state,omitempty"`      // "ON" or "OFF"
	Brightness *int    `json:"brightness,omitempty"`  // 0-100
}

// AVCStateUpdate represents a parsed state update from the avc WebSocket.
type AVCStateUpdate struct {
	State string // e.g. "module.50000", "scene.4"
	Value string // e.g. "75,100,X,-1", "1"
}

// --- Helpers ---

// Slugify converts a human-readable name to a slug for MQTT topics.
// Lowercase, spaces → _, apostrophes removed.
func Slugify(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == ' ':
			b.WriteByte('_')
		case r == '\'':
			// skip apostrophes
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_':
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// ParseDeviceAddress converts a hex address string (e.g. "005") to an int.
func ParseDeviceAddress(hex string) (int, error) {
	addr, err := strconv.ParseInt(hex, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid device address %q: %w", hex, err)
	}
	return int(addr), nil
}

// ComputeHexAddr computes the full hex address from a device address and load/button offset.
// Formula: (deviceAddr << 16) | (offset & 0x3FF)
func ComputeHexAddr(deviceAddr int, offset int) int {
	return (deviceAddr << 16) | (offset & 0x3FF)
}

// ComputeSwitchAddr computes the switch address for a button.
// Formula: (deviceAddr << 16) | ((buttonIndex - 1) & 0x3FF)
func ComputeSwitchAddr(deviceAddr int, buttonIndex int) int {
	return (deviceAddr << 16) | ((buttonIndex - 1) & 0x3FF)
}
