package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("failed to read fixture %s: %v", name, err)
	}
	return data
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/config/v1/rooms", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "rooms.json"))
	})
	mux.HandleFunc("/config/v1/lighting/loads", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "loads.json"))
	})
	mux.HandleFunc("/config/v1/lighting/devices", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "devices.json"))
	})
	mux.HandleFunc("/config/v1/lighting/devices/642DA5F1-EB43-4F43-957D-1858599AC144", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "device_with_buttons.json"))
	})
	mux.HandleFunc("/states/", func(w http.ResponseWriter, r *http.Request) {
		stateName := r.URL.Path[len("/states/"):]
		w.Header().Set("Content-Type", "application/json")
		resp := map[string][]string{"data": {stateName + "_value"}}
		json.NewEncoder(w).Encode(resp)
	})
	return httptest.NewServer(mux)
}

func apiFromTestServer(ts *httptest.Server) *SavantAPI {
	api := NewSavantAPI("127.0.0.1", 3060)
	api.baseURL = ts.URL
	return api
}

func TestFetchRooms(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	api := apiFromTestServer(ts)

	rooms, err := api.FetchRooms(context.Background())
	if err != nil {
		t.Fatalf("FetchRooms error: %v", err)
	}

	if got := len(rooms); got != 20 {
		t.Errorf("expected 20 rooms, got %d", got)
	}

	// Verify a known room
	found := false
	for _, r := range rooms {
		if r.Name == "Den" {
			found = true
			if r.RoomID != "2AE47528-2963-45EB-A3B5-7E2ECF438891" {
				t.Errorf("Den room ID = %s, want 2AE47528-2963-45EB-A3B5-7E2ECF438891", r.RoomID)
			}
			break
		}
	}
	if !found {
		t.Error("Den room not found")
	}
}

func TestFetchLoads(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	api := apiFromTestServer(ts)

	loads, err := api.FetchLoads(context.Background())
	if err != nil {
		t.Fatalf("FetchLoads error: %v", err)
	}

	if got := len(loads); got != 40 {
		t.Errorf("expected 40 loads, got %d", got)
	}
}

func TestFetchDevices(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	api := apiFromTestServer(ts)

	devices, err := api.FetchDevices(context.Background())
	if err != nil {
		t.Fatalf("FetchDevices error: %v", err)
	}

	if got := len(devices); got != 41 {
		t.Errorf("expected 41 devices, got %d", got)
	}
}

func TestFetchDevice(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	api := apiFromTestServer(ts)

	device, err := api.FetchDevice(context.Background(), "642DA5F1-EB43-4F43-957D-1858599AC144")
	if err != nil {
		t.Fatalf("FetchDevice error: %v", err)
	}

	if device.Name != "Den Hallway" {
		t.Errorf("device name = %s, want Den Hallway", device.Name)
	}
	if len(device.Loads) != 1 {
		t.Errorf("expected 1 load, got %d", len(device.Loads))
	}
	if len(device.Buttons) != 6 {
		t.Errorf("expected 6 buttons, got %d", len(device.Buttons))
	}
}

func TestFetchState(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	api := apiFromTestServer(ts)

	val, err := api.FetchState(context.Background(), "Den.BrightnessLevel")
	if err != nil {
		t.Fatalf("FetchState error: %v", err)
	}
	if val != "Den.BrightnessLevel_value" {
		t.Errorf("state value = %s, want Den.BrightnessLevel_value", val)
	}
}

func TestFetchStates(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()
	api := apiFromTestServer(ts)

	names := []string{"Den.BrightnessLevel", "Kitchen.PowerState"}
	states, err := api.FetchStates(context.Background(), names)
	if err != nil {
		t.Fatalf("FetchStates error: %v", err)
	}

	if len(states) != 2 {
		t.Errorf("expected 2 states, got %d", len(states))
	}
	for _, name := range names {
		expected := name + "_value"
		if states[name] != expected {
			t.Errorf("states[%s] = %s, want %s", name, states[name], expected)
		}
	}
}

func TestFetchError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	api := apiFromTestServer(ts)

	_, err := api.FetchRooms(context.Background())
	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

func loadFixtureInto(t *testing.T, name string, dest interface{}) {
	t.Helper()
	data := loadFixture(t, name)
	if err := json.Unmarshal(data, dest); err != nil {
		t.Fatalf("failed to unmarshal fixture %s: %v", name, err)
	}
}

func TestBuildEntities(t *testing.T) {
	var rooms []Room
	var loads []Load
	var devices []Device
	var deviceWithButtons Device

	loadFixtureInto(t, "rooms.json", &rooms)
	loadFixtureInto(t, "loads.json", &loads)
	loadFixtureInto(t, "devices.json", &devices)
	loadFixtureInto(t, "device_with_buttons.json", &deviceWithButtons)

	// Merge the device-with-buttons into the devices list by replacing
	// the matching entry so BuildEntities sees the button data.
	for i, d := range devices {
		if d.DeviceID == deviceWithButtons.DeviceID {
			devices[i] = deviceWithButtons
			break
		}
	}

	entities, err := BuildEntities(rooms, loads, devices)
	if err != nil {
		t.Fatalf("BuildEntities error: %v", err)
	}

	// Should produce one entity per load
	if got := len(entities); got != 40 {
		t.Fatalf("expected 40 entities, got %d", got)
	}

	// Find the Den Lights entity
	var denLights *LightEntity
	var denCloset *LightEntity
	for i := range entities {
		if entities[i].RoomName == "Den" && entities[i].Name == "Lights" {
			denLights = &entities[i]
		}
		if entities[i].RoomName == "Den" && entities[i].Name == "Closet" {
			denCloset = &entities[i]
		}
	}

	t.Run("DenLights", func(t *testing.T) {
		if denLights == nil {
			t.Fatal("Den Lights entity not found")
		}

		// Device address "005" = 5, offset 0
		// HexAddr = (5 << 16) | 0 = 0x50000 = 327680
		expectedHexAddr := 0x50000
		if denLights.HexAddr != expectedHexAddr {
			t.Errorf("HexAddr = 0x%X, want 0x%X", denLights.HexAddr, expectedHexAddr)
		}

		if !denLights.FollowDaylight {
			t.Error("FollowDaylight = false, want true")
		}

		if !denLights.IsDimmable {
			t.Error("IsDimmable = false, want true")
		}

		if denLights.UniqueID != "savant_load_005_0" {
			t.Errorf("UniqueID = %s, want savant_load_005_0", denLights.UniqueID)
		}

		if denLights.RoomSlug != "den" {
			t.Errorf("RoomSlug = %s, want den", denLights.RoomSlug)
		}

		if denLights.DeviceModel != "ECHO Adaptive phase" {
			t.Errorf("DeviceModel = %s, want ECHO Adaptive phase", denLights.DeviceModel)
		}

		// natural light toggle: button index 3, device address 5
		// switchAddr = (5 << 16) | ((3-1) & 0x3FF) = 0x50002 = 327682
		expectedToggleAddr := 0x50002
		if denLights.DaylightToggleAddr != expectedToggleAddr {
			t.Errorf("DaylightToggleAddr = 0x%X, want 0x%X", denLights.DaylightToggleAddr, expectedToggleAddr)
		}
	})

	t.Run("DenCloset", func(t *testing.T) {
		if denCloset == nil {
			t.Fatal("Den Closet entity not found")
		}

		if denCloset.IsDimmable {
			t.Error("IsDimmable = true, want false (type=1 is switch)")
		}

		// Closet has followDaylight=false, so DaylightToggleAddr should be -1
		if denCloset.DaylightToggleAddr != -1 {
			t.Errorf("DaylightToggleAddr = %d, want -1", denCloset.DaylightToggleAddr)
		}
	})

	t.Run("DaylightToggleForLowerHall", func(t *testing.T) {
		// The device_with_buttons has a button targeting Lower Hall
		// (roomID 3294147F-E4F0-4CAA-84BE-71E7C09EA97E) at index 1.
		// switchAddr = (5 << 16) | ((1-1) & 0x3FF) = 0x50000
		var lowerHallEntity *LightEntity
		for i := range entities {
			if entities[i].RoomName == "Lower Hall" {
				lowerHallEntity = &entities[i]
				break
			}
		}
		if lowerHallEntity == nil {
			t.Fatal("Lower Hall entity not found")
		}

		if !lowerHallEntity.FollowDaylight {
			t.Fatal("Lower Hall FollowDaylight = false, want true")
		}

		expectedToggleAddr := 0x50000
		if lowerHallEntity.DaylightToggleAddr != expectedToggleAddr {
			t.Errorf("DaylightToggleAddr = 0x%X, want 0x%X", lowerHallEntity.DaylightToggleAddr, expectedToggleAddr)
		}
	})

	t.Run("NonFollowDaylightNoDaylightToggle", func(t *testing.T) {
		// Verify entities with followDaylight=false always have DaylightToggleAddr=-1
		for _, e := range entities {
			if !e.FollowDaylight && e.DaylightToggleAddr != -1 {
				t.Errorf("entity %s has FollowDaylight=false but DaylightToggleAddr=%d (want -1)",
					e.UniqueID, e.DaylightToggleAddr)
			}
		}
	})
}

func TestBuildEntitiesMissingDevice(t *testing.T) {
	rooms := []Room{{RoomID: "room1", Name: "Test"}}
	loads := []Load{{LoadID: "load1", DeviceID: "missing", RoomID: "room1"}}
	devices := []Device{}

	_, err := BuildEntities(rooms, loads, devices)
	if err == nil {
		t.Error("expected error for missing device, got nil")
	}
}

func TestBuildEntitiesMissingRoom(t *testing.T) {
	rooms := []Room{}
	loads := []Load{{LoadID: "load1", DeviceID: "dev1", RoomID: "missing"}}
	devices := []Device{{DeviceID: "dev1", Address: "001"}}

	_, err := BuildEntities(rooms, loads, devices)
	if err == nil {
		t.Error("expected error for missing room, got nil")
	}
}
