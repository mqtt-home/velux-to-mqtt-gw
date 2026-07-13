package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/philipparndt/go-logger"
	gwconfig "github.com/philipparndt/mqtt-gateway/config"
)

// MQTTConfig holds MQTT broker connection settings.
type MQTTConfig struct {
	URL      string `json:"url"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Retain   bool   `json:"retain"`
	ClientID string `json:"client-id,omitempty"`
	QoS      byte   `json:"qos"`
	Topic    string `json:"topic,omitempty"`
}

// ToGatewayConfig converts to the shared mqtt-gateway config type.
func (m MQTTConfig) ToGatewayConfig() gwconfig.MQTTConfig {
	return gwconfig.MQTTConfig{
		URL:      m.URL,
		Retain:   m.Retain,
		Topic:    m.Topic,
		QoS:      m.QoS,
		Username: m.Username,
		Password: m.Password,
	}
}

// VeluxConfig holds KLF200 gateway connection settings.
type VeluxConfig struct {
	Host     string `json:"host"`
	Password string `json:"password"`
}

// HomeAssistantConfig holds Home Assistant integration settings.
type HomeAssistantConfig struct {
	Prefix       string `json:"prefix"`
	InvertAwning bool   `json:"invert-awning"`
}

// RestartConfig controls the watchdog / restart behaviour.
type RestartConfig struct {
	RestartInterval     int  `json:"restart-interval"`
	HealthCheckInterval int  `json:"health-check-interval"`
	RestartOnError      bool `json:"restart-on-error"`
}

// Config is the top-level configuration for the velux-to-mqtt-gw bridge.
type Config struct {
	MQTT          MQTTConfig          `json:"mqtt"`
	Velux         VeluxConfig         `json:"velux"`
	HomeAssistant HomeAssistantConfig `json:"homeassistant"`
	Restart       RestartConfig       `json:"restart"`
	LogLevel      string              `json:"loglevel,omitempty"`
}

// ApplyDefaults fills in unset optional fields with their documented defaults.
// Boolean defaults that are true (retain, restart-on-error) are pre-seeded
// before JSON unmarshalling in LoadConfig; this function covers numeric and
// string defaults.
func ApplyDefaults(c *Config) {
	if c.MQTT.QoS == 0 {
		c.MQTT.QoS = 1
	}
	if c.Restart.RestartInterval == 0 {
		c.Restart.RestartInterval = 24
	}
	if c.Restart.HealthCheckInterval == 0 {
		c.Restart.HealthCheckInterval = 300
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	// HomeAssistant.Prefix defaults to "" and InvertAwning to false — both zero
	// values, so no explicit action required.
}

// ReplaceEnvVariables substitutes ${NAME} placeholders with environment
// variable values; unset variables become empty strings.
func ReplaceEnvVariables(input []byte) []byte {
	return gwconfig.ReplaceEnvVariables(input)
}

var (
	mu  sync.RWMutex
	cfg Config
)

// LoadConfig reads a JSON config file from path, substitutes ${ENV} variables,
// applies defaults, validates required fields, and returns the parsed Config.
func LoadConfig(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("config path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}

	data = ReplaceEnvVariables(data)

	// Pre-seed boolean defaults that are true so that explicit false overrides
	// them correctly after JSON unmarshalling.
	c := Config{
		MQTT: MQTTConfig{
			Retain: true,
		},
		Restart: RestartConfig{
			RestartOnError: true,
		},
	}

	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}

	ApplyDefaults(&c)

	if err := validate(&c); err != nil {
		return Config{}, err
	}

	mu.Lock()
	cfg = c
	mu.Unlock()

	logger.Debug("Config loaded", "file", path, "loglevel", c.LogLevel)
	return c, nil
}

// validate returns a descriptive error when any required field is absent.
func validate(c *Config) error {
	if c.MQTT.URL == "" {
		return errors.New("required field missing: mqtt.url")
	}
	if c.Velux.Host == "" {
		return errors.New("required field missing: velux.host")
	}
	if c.Velux.Password == "" {
		return errors.New("required field missing: velux.password")
	}
	return nil
}

// Get returns the currently loaded config (zero value if none loaded yet).
func Get() Config {
	mu.RLock()
	defer mu.RUnlock()
	return cfg
}
