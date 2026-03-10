package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	content := `
savant:
  host: "192.168.1.100"
  rest_port: 3060
  avc_port: 8480
  config_name: "Farah SEA"
mqtt:
  broker: "tcp://192.168.1.10:1883"
  username: "user"
  password: "pass"
  topic_prefix: "myprefix"
homeassistant:
  discovery_prefix: "ha"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Savant.Host != "192.168.1.100" {
		t.Errorf("Savant.Host = %q, want 192.168.1.100", cfg.Savant.Host)
	}
	if cfg.Savant.RESTPort != 3060 {
		t.Errorf("Savant.RESTPort = %d, want 3060", cfg.Savant.RESTPort)
	}
	if cfg.MQTT.Broker != "tcp://192.168.1.10:1883" {
		t.Errorf("MQTT.Broker = %q", cfg.MQTT.Broker)
	}
	if cfg.MQTT.Username != "user" {
		t.Errorf("MQTT.Username = %q, want user", cfg.MQTT.Username)
	}
	if cfg.MQTT.Password != "pass" {
		t.Errorf("MQTT.Password = %q, want pass", cfg.MQTT.Password)
	}
	if cfg.MQTT.TopicPrefix != "myprefix" {
		t.Errorf("MQTT.TopicPrefix = %q, want myprefix", cfg.MQTT.TopicPrefix)
	}
	if cfg.HA.DiscoveryPrefix != "ha" {
		t.Errorf("HA.DiscoveryPrefix = %q, want ha", cfg.HA.DiscoveryPrefix)
	}
	if cfg.Savant.ConfigName != "Farah SEA" {
		t.Errorf("Savant.ConfigName = %q, want Farah SEA", cfg.Savant.ConfigName)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	content := `
mqtt:
  broker: "tcp://localhost:1883"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Savant.Host != "127.0.0.1" {
		t.Errorf("Savant.Host = %q, want 127.0.0.1 (default)", cfg.Savant.Host)
	}
	if cfg.Savant.RESTPort != 3062 {
		t.Errorf("Savant.RESTPort = %d, want 3062 (default)", cfg.Savant.RESTPort)
	}
	if cfg.Savant.AVCPort != 8480 {
		t.Errorf("Savant.AVCPort = %d, want 8480 (default)", cfg.Savant.AVCPort)
	}
	if cfg.MQTT.TopicPrefix != "savant" {
		t.Errorf("MQTT.TopicPrefix = %q, want savant (default)", cfg.MQTT.TopicPrefix)
	}
	if cfg.HA.DiscoveryPrefix != "homeassistant" {
		t.Errorf("HA.DiscoveryPrefix = %q, want homeassistant (default)", cfg.HA.DiscoveryPrefix)
	}
}

func TestLoadConfigEnvOverride(t *testing.T) {
	content := `
mqtt:
  broker: "tcp://localhost:1883"
  password: "from_file"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("MQTT_PASSWORD", "from_env")

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.MQTT.Password != "from_env" {
		t.Errorf("MQTT.Password = %q, want from_env (env override)", cfg.MQTT.Password)
	}
}

func TestLoadConfigMissingBroker(t *testing.T) {
	content := `
port: 9090
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Fatal("expected error for missing mqtt.broker, got nil")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
