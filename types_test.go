package main

import (
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Den", "den"},
		{"Master Bath", "master_bath"},
		{"Evelyn's Room", "evelyns_room"},
		{"Kids' Bath", "kids_bath"},
		{"Julian's Room", "julians_room"},
		{"Upper Hall", "upper_hall"},
		{"Bernardo's Night Stand", "bernardos_night_stand"},
		{"Guest Bed", "guest_bed"},
		{"Lower Hall", "lower_hall"},
		{"JustHereForDaylightMode", "justherefordaylightmode"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDeviceAddress(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"001", 1},
		{"005", 5},
		{"00a", 10},
		{"021", 33},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDeviceAddress(tt.input)
			if err != nil {
				t.Fatalf("ParseDeviceAddress(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseDeviceAddress(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestComputeHexAddr(t *testing.T) {
	// device "005" (addr=5), load offset 0 → 0x50000
	got := ComputeHexAddr(5, 0)
	if got != 0x50000 {
		t.Errorf("ComputeHexAddr(5, 0) = 0x%x, want 0x50000", got)
	}

	// device "005", load offset 1 → 0x50001
	got = ComputeHexAddr(5, 1)
	if got != 0x50001 {
		t.Errorf("ComputeHexAddr(5, 1) = 0x%x, want 0x50001", got)
	}
}

func TestComputeSwitchAddr(t *testing.T) {
	// device "005" (addr=5), button index 3 → (5 << 16) | (3-1) = 0x50002
	got := ComputeSwitchAddr(5, 3)
	if got != 0x50002 {
		t.Errorf("ComputeSwitchAddr(5, 3) = 0x%x, want 0x50002", got)
	}

	// device "005", button index 5 → (5 << 16) | (5-1) = 0x50004
	got = ComputeSwitchAddr(5, 5)
	if got != 0x50004 {
		t.Errorf("ComputeSwitchAddr(5, 5) = 0x%x, want 0x50004", got)
	}

	// button index 1 → offset 0
	got = ComputeSwitchAddr(5, 1)
	if got != 0x50000 {
		t.Errorf("ComputeSwitchAddr(5, 1) = 0x%x, want 0x50000", got)
	}
}

func TestLightEntityTopics(t *testing.T) {
	e := LightEntity{
		UniqueID: "savant_load_005_0",
		RoomSlug: "den",
		LoadSlug: "lights",
	}

	if got := e.StateTopic("savant"); got != "savant/den/lights/light/state" {
		t.Errorf("StateTopic = %q, want savant/den/lights/light/state", got)
	}
	if got := e.CommandTopic("savant"); got != "savant/den/lights/light/set" {
		t.Errorf("CommandTopic = %q, want savant/den/lights/light/set", got)
	}
	if got := e.DiscoveryTopic("homeassistant"); got != "homeassistant/light/savant_load_005_0/config" {
		t.Errorf("DiscoveryTopic = %q, want homeassistant/light/savant_load_005_0/config", got)
	}
}
