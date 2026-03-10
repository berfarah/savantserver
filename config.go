package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Savant SavantConfig `yaml:"savant"`
	MQTT   MQTTConfig   `yaml:"mqtt"`
	HA     HAConfig     `yaml:"homeassistant"`
}

type SavantConfig struct {
	Host       string `yaml:"host"`
	RESTPort   int    `yaml:"rest_port"`
	AVCPort    int    `yaml:"avc_port"`
	ConfigName string `yaml:"config_name"` // Savant configuration name (e.g. "Farah SEA"), used for per-load state queries
}

type MQTTConfig struct {
	Broker      string `yaml:"broker"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	TopicPrefix string `yaml:"topic_prefix"`
}

type HAConfig struct {
	DiscoveryPrefix string `yaml:"discovery_prefix"`
}

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		Savant: SavantConfig{
			Host:     "127.0.0.1",
			RESTPort: 3062,
			AVCPort:  8480,
		},
		MQTT: MQTTConfig{
			TopicPrefix: "savant",
		},
		HA: HAConfig{
			DiscoveryPrefix: "homeassistant",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Environment variable override for MQTT password
	if envPass := os.Getenv("MQTT_PASSWORD"); envPass != "" {
		cfg.MQTT.Password = envPass
	}

	if cfg.MQTT.Broker == "" {
		return nil, fmt.Errorf("mqtt.broker is required in config")
	}

	return cfg, nil
}
