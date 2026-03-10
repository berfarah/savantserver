package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClient wraps the paho MQTT client with Savant-specific publishing
// and subscription logic for Home Assistant integration.
type MQTTClient struct {
	client      mqtt.Client
	topicPrefix string
	haPrefix    string
	logger      *log.Logger
}

// NewMQTTClient creates an MQTT client configured for the Savant bridge.
func NewMQTTClient(cfg MQTTConfig, haPrefix string) *MQTTClient {
	logger := log.New(os.Stderr, "[mqtt] ", log.LstdFlags)

	m := &MQTTClient{
		topicPrefix: cfg.TopicPrefix,
		haPrefix:    haPrefix,
		logger:      logger,
	}

	statusTopic := cfg.TopicPrefix + "/status"

	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker)
	opts.SetClientID("savantserver")
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
	}
	if cfg.Password != "" {
		opts.SetPassword(cfg.Password)
	}

	// Last Will and Testament — broker publishes "offline" if we disconnect unexpectedly
	opts.SetWill(statusTopic, "offline", 1, true)

	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		logger.Println("MQTT connected")
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		logger.Printf("MQTT connection lost: %v", err)
	})

	m.client = mqtt.NewClient(opts)
	return m
}

// Connect establishes the MQTT connection. Returns an error if the
// connection cannot be established within 10 seconds.
func (m *MQTTClient) Connect(ctx context.Context) error {
	token := m.client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("MQTT connect timed out")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT connect: %w", err)
	}
	return nil
}

// Close publishes an offline status message and disconnects.
func (m *MQTTClient) Close() {
	statusTopic := m.topicPrefix + "/status"
	token := m.client.Publish(statusTopic, 1, true, "offline")
	token.WaitTimeout(2 * time.Second)
	m.client.Disconnect(1000)
}

// PublishAvailability publishes the bridge availability status.
func (m *MQTTClient) PublishAvailability(online bool) error {
	payload := "offline"
	if online {
		payload = "online"
	}
	statusTopic := m.topicPrefix + "/status"
	token := m.client.Publish(statusTopic, 1, true, payload)
	token.Wait()
	return token.Error()
}

// PublishDiscovery sends HA MQTT Discovery config messages for all entities.
func (m *MQTTClient) PublishDiscovery(entities []LightEntity) error {
	for i := range entities {
		entity := &entities[i]
		payload, err := buildDiscoveryPayload(entity, m.topicPrefix, m.haPrefix)
		if err != nil {
			return fmt.Errorf("building discovery payload for %s: %w", entity.UniqueID, err)
		}
		topic := entity.DiscoveryTopic(m.haPrefix)
		token := m.client.Publish(topic, 1, true, payload)
		token.Wait()
		if err := token.Error(); err != nil {
			return fmt.Errorf("publishing discovery for %s: %w", entity.UniqueID, err)
		}
	}
	m.logger.Printf("Published discovery for %d entities", len(entities))
	return nil
}

// PublishState publishes the current state of a light entity.
func (m *MQTTClient) PublishState(entity *LightEntity, state LightState) error {
	payload, err := buildStatePayload(entity, state)
	if err != nil {
		return fmt.Errorf("building state payload for %s: %w", entity.UniqueID, err)
	}
	topic := entity.StateTopic(m.topicPrefix)
	token := m.client.Publish(topic, 0, false, payload)
	token.Wait()
	return token.Error()
}

// SubscribeCommands subscribes to command topics for all lights and dispatches
// parsed commands to the provided handler.
func (m *MQTTClient) SubscribeCommands(handler func(entityID string, cmd LightCommand)) error {
	topic := m.topicPrefix + "/+/+/light/set"
	token := m.client.Subscribe(topic, 1, func(_ mqtt.Client, msg mqtt.Message) {
		entityID, err := parseCommandTopic(msg.Topic(), m.topicPrefix)
		if err != nil {
			m.logger.Printf("Ignoring message on %s: %v", msg.Topic(), err)
			return
		}

		var cmd LightCommand
		if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
			m.logger.Printf("Invalid command payload on %s: %v", msg.Topic(), err)
			return
		}

		handler(entityID, cmd)
	})
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("subscribing to %s: %w", topic, err)
	}
	m.logger.Printf("Subscribed to %s", topic)
	return nil
}

// --- Serialization helpers (exported for testing) ---

// discoveryDevice is the "device" block in an HA discovery payload.
type discoveryDevice struct {
	Identifiers   []string `json:"identifiers"`
	Name          string   `json:"name"`
	Manufacturer  string   `json:"manufacturer"`
	Model         string   `json:"model"`
	SuggestedArea string   `json:"suggested_area"`
}

// discoveryPayload is the HA MQTT Discovery config for a light entity.
type discoveryPayload struct {
	Schema            string          `json:"schema"`
	Name              string          `json:"name"`
	UniqueID          string          `json:"unique_id"`
	ObjectID          string          `json:"object_id"`
	Brightness        bool            `json:"brightness,omitempty"`
	BrightnessScale   int             `json:"brightness_scale,omitempty"`
	StateTopic        string          `json:"state_topic"`
	CommandTopic      string          `json:"command_topic"`
	AvailabilityTopic string          `json:"availability_topic"`
	PayloadAvailable  string          `json:"payload_available"`
	PayloadNotAvail   string          `json:"payload_not_available"`
	Device            discoveryDevice `json:"device"`
}

// buildDiscoveryPayload generates the HA MQTT Discovery JSON for an entity.
func buildDiscoveryPayload(entity *LightEntity, topicPrefix string, haPrefix string) ([]byte, error) {
	p := discoveryPayload{
		Schema:            "json",
		Name:              entity.Name,
		UniqueID:          entity.UniqueID,
		ObjectID:          entity.UniqueID,
		StateTopic:        entity.StateTopic(topicPrefix),
		CommandTopic:      entity.CommandTopic(topicPrefix),
		AvailabilityTopic: topicPrefix + "/status",
		PayloadAvailable:  "online",
		PayloadNotAvail:   "offline",
		Device: discoveryDevice{
			Identifiers:   []string{entity.UniqueID},
			Name:          entity.RoomName + " " + entity.Name,
			Manufacturer:  "Savant",
			Model:         entity.DeviceModel,
			SuggestedArea: entity.RoomName,
		},
	}

	if entity.IsDimmable {
		p.Brightness = true
		p.BrightnessScale = 100
	}

	return json.Marshal(p)
}

// statePayload is the JSON state message published for a light.
type statePayload struct {
	State      string `json:"state"`
	Brightness *int   `json:"brightness,omitempty"`
}

// buildStatePayload generates the JSON state payload for a light entity.
func buildStatePayload(entity *LightEntity, state LightState) ([]byte, error) {
	p := statePayload{}
	if state.On {
		p.State = "ON"
	} else {
		p.State = "OFF"
	}

	if entity.IsDimmable && state.Brightness >= 0 {
		b := state.Brightness
		if !state.On {
			b = 0
		}
		p.Brightness = &b
	}

	return json.Marshal(p)
}

// parseCommandTopic extracts the entityID (room_slug/load_slug) from a
// command topic like "<prefix>/<room>/<load>/light/set".
func parseCommandTopic(topic string, prefix string) (string, error) {
	if !strings.HasPrefix(topic, prefix+"/") {
		return "", fmt.Errorf("topic %q does not start with prefix %q", topic, prefix)
	}
	rest := topic[len(prefix)+1:] // strip "<prefix>/"
	parts := strings.Split(rest, "/")
	// expect: <room>/<load>/light/set → 4 parts
	if len(parts) != 4 || parts[2] != "light" || parts[3] != "set" {
		return "", fmt.Errorf("unexpected topic format: %q", topic)
	}
	return parts[0] + "/" + parts[1], nil
}
