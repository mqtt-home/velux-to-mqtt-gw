package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- ReplaceEnvVariables ----

func TestReplaceEnvVariables_Set(t *testing.T) {
	t.Setenv("VELUX_PW", "s3cr3t")
	got := string(ReplaceEnvVariables([]byte(`{"password":"${VELUX_PW}"}`)))
	want := `{"password":"s3cr3t"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReplaceEnvVariables_Unset(t *testing.T) {
	_ = os.Unsetenv("VELUX_NOPE")
	got := string(ReplaceEnvVariables([]byte(`{"x":"${VELUX_NOPE}"}`)))
	want := `{"x":""}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReplaceEnvVariables_Multiple(t *testing.T) {
	t.Setenv("A", "alpha")
	t.Setenv("B", "beta")
	got := string(ReplaceEnvVariables([]byte(`{"a":"${A}","b":"${B}","c":"${A}"}`)))
	want := `{"a":"alpha","b":"beta","c":"alpha"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestReplaceEnvVariables_NoVars(t *testing.T) {
	in := `{"x": "plain"}`
	got := string(ReplaceEnvVariables([]byte(in)))
	if got != in {
		t.Errorf("got %q, want %q", got, in)
	}
}

// ---- ApplyDefaults ----

func TestApplyDefaults_AllDefaults(t *testing.T) {
	c := Config{}
	ApplyDefaults(&c)

	if c.MQTT.QoS != 1 {
		t.Errorf("MQTT.QoS = %d, want 1", c.MQTT.QoS)
	}
	if c.Restart.RestartInterval != 24 {
		t.Errorf("RestartInterval = %d, want 24", c.Restart.RestartInterval)
	}
	if c.Restart.HealthCheckInterval != 300 {
		t.Errorf("HealthCheckInterval = %d, want 300", c.Restart.HealthCheckInterval)
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", c.LogLevel)
	}
	if c.HomeAssistant.Prefix != "" {
		t.Errorf("HomeAssistant.Prefix = %q, want empty", c.HomeAssistant.Prefix)
	}
	if c.HomeAssistant.InvertAwning {
		t.Error("HomeAssistant.InvertAwning should default to false")
	}
}

func TestApplyDefaults_DoesNotOverrideExplicit(t *testing.T) {
	c := Config{
		MQTT:          MQTTConfig{QoS: 2},
		Restart:       RestartConfig{RestartInterval: 48, HealthCheckInterval: 60},
		HomeAssistant: HomeAssistantConfig{Prefix: "ha", InvertAwning: true},
		LogLevel:      "debug",
	}
	ApplyDefaults(&c)

	if c.MQTT.QoS != 2 {
		t.Errorf("QoS = %d, want 2", c.MQTT.QoS)
	}
	if c.Restart.RestartInterval != 48 {
		t.Errorf("RestartInterval = %d, want 48", c.Restart.RestartInterval)
	}
	if c.Restart.HealthCheckInterval != 60 {
		t.Errorf("HealthCheckInterval = %d, want 60", c.Restart.HealthCheckInterval)
	}
	if c.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", c.LogLevel)
	}
	if c.HomeAssistant.Prefix != "ha" {
		t.Errorf("HomeAssistant.Prefix = %q, want ha", c.HomeAssistant.Prefix)
	}
	if !c.HomeAssistant.InvertAwning {
		t.Error("HomeAssistant.InvertAwning should stay true")
	}
}

// ---- LoadConfig: defaults applied ----

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func minimalConfig() string {
	return `{
		"mqtt":  {"url": "tcp://localhost:1883"},
		"velux": {"host": "192.168.1.1", "password": "secret"}
	}`
}

func TestLoadConfig_DefaultsApplied(t *testing.T) {
	c, err := LoadConfig(writeConfig(t, minimalConfig()))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if !c.MQTT.Retain {
		t.Error("MQTT.Retain should default to true")
	}
	if c.MQTT.QoS != 1 {
		t.Errorf("MQTT.QoS = %d, want 1", c.MQTT.QoS)
	}
	if c.Restart.RestartInterval != 24 {
		t.Errorf("RestartInterval = %d, want 24", c.Restart.RestartInterval)
	}
	if c.Restart.HealthCheckInterval != 300 {
		t.Errorf("HealthCheckInterval = %d, want 300", c.Restart.HealthCheckInterval)
	}
	if !c.Restart.RestartOnError {
		t.Error("RestartOnError should default to true")
	}
	if c.HomeAssistant.Prefix != "" {
		t.Errorf("HomeAssistant.Prefix = %q, want empty", c.HomeAssistant.Prefix)
	}
	if c.HomeAssistant.InvertAwning {
		t.Error("InvertAwning should default to false")
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", c.LogLevel)
	}
}

// ---- LoadConfig: explicit overrides ----

func TestLoadConfig_ExplicitOverridesDefaults(t *testing.T) {
	body := `{
		"mqtt":  {"url": "tcp://broker:1883", "retain": false, "qos": 2, "topic": "velux"},
		"velux": {"host": "10.0.0.1", "password": "pw"},
		"homeassistant": {"prefix": "ha", "invert-awning": true},
		"restart": {"restart-interval": 12, "health-check-interval": 60, "restart-on-error": false},
		"loglevel": "debug"
	}`
	c, err := LoadConfig(writeConfig(t, body))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if c.MQTT.Retain {
		t.Error("Retain should be false")
	}
	if c.MQTT.QoS != 2 {
		t.Errorf("QoS = %d, want 2", c.MQTT.QoS)
	}
	if c.MQTT.Topic != "velux" {
		t.Errorf("Topic = %q, want velux", c.MQTT.Topic)
	}
	if c.HomeAssistant.Prefix != "ha" {
		t.Errorf("Prefix = %q, want ha", c.HomeAssistant.Prefix)
	}
	if !c.HomeAssistant.InvertAwning {
		t.Error("InvertAwning should be true")
	}
	if c.Restart.RestartInterval != 12 {
		t.Errorf("RestartInterval = %d, want 12", c.Restart.RestartInterval)
	}
	if c.Restart.HealthCheckInterval != 60 {
		t.Errorf("HealthCheckInterval = %d, want 60", c.Restart.HealthCheckInterval)
	}
	if c.Restart.RestartOnError {
		t.Error("RestartOnError should be false")
	}
	if c.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", c.LogLevel)
	}
}

// ---- LoadConfig: env substitution ----

func TestLoadConfig_EnvSubstitution_Set(t *testing.T) {
	t.Setenv("VELUX_HOST", "gw.local")
	t.Setenv("VELUX_PASSWORD", "topsecret")

	body := `{
		"mqtt":  {"url": "tcp://localhost:1883"},
		"velux": {"host": "${VELUX_HOST}", "password": "${VELUX_PASSWORD}"}
	}`
	c, err := LoadConfig(writeConfig(t, body))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if c.Velux.Host != "gw.local" {
		t.Errorf("Velux.Host = %q, want gw.local", c.Velux.Host)
	}
	if c.Velux.Password != "topsecret" {
		t.Errorf("Velux.Password = %q, want topsecret", c.Velux.Password)
	}
}

func TestLoadConfig_EnvSubstitution_Unset(t *testing.T) {
	_ = os.Unsetenv("VELUX_MISSING")

	// Unset var becomes empty, triggering required-field validation for velux.password.
	body := `{
		"mqtt":  {"url": "tcp://localhost:1883"},
		"velux": {"host": "gw.local", "password": "${VELUX_MISSING}"}
	}`
	_, err := LoadConfig(writeConfig(t, body))
	if err == nil {
		t.Fatal("expected error for empty required field")
	}
	if !strings.Contains(err.Error(), "velux.password") {
		t.Errorf("error should mention velux.password, got: %v", err)
	}
}

func TestLoadConfig_EnvSubstitution_Multiple(t *testing.T) {
	t.Setenv("MQTT_URL", "tcp://broker:1883")
	t.Setenv("GW_HOST", "192.168.0.10")
	t.Setenv("GW_PW", "pass")

	body := `{
		"mqtt":  {"url": "${MQTT_URL}"},
		"velux": {"host": "${GW_HOST}", "password": "${GW_PW}"}
	}`
	c, err := LoadConfig(writeConfig(t, body))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if c.MQTT.URL != "tcp://broker:1883" {
		t.Errorf("MQTT.URL = %q, want tcp://broker:1883", c.MQTT.URL)
	}
	if c.Velux.Host != "192.168.0.10" {
		t.Errorf("Velux.Host = %q, want 192.168.0.10", c.Velux.Host)
	}
}

// ---- LoadConfig: missing required fields ----

func TestLoadConfig_MissingMQTTURL(t *testing.T) {
	body := `{"velux": {"host": "gw.local", "password": "pw"}}`
	_, err := LoadConfig(writeConfig(t, body))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "mqtt.url") {
		t.Errorf("error should name mqtt.url, got: %v", err)
	}
}

func TestLoadConfig_MissingVeluxHost(t *testing.T) {
	body := `{"mqtt": {"url": "tcp://localhost:1883"}, "velux": {"password": "pw"}}`
	_, err := LoadConfig(writeConfig(t, body))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "velux.host") {
		t.Errorf("error should name velux.host, got: %v", err)
	}
}

func TestLoadConfig_MissingVeluxPassword(t *testing.T) {
	body := `{"mqtt": {"url": "tcp://localhost:1883"}, "velux": {"host": "gw.local"}}`
	_, err := LoadConfig(writeConfig(t, body))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "velux.password") {
		t.Errorf("error should name velux.password, got: %v", err)
	}
}

// ---- LoadConfig: file not found ----

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// ---- LoadConfig: sample round-trip ----

func TestLoadConfig_SampleRoundTrip(t *testing.T) {
	body := `{
		"mqtt": {
			"url": "tcp://mqtt.local:1883",
			"username": "user",
			"password": "pass",
			"retain": true,
			"client-id": "velux-gw",
			"qos": 1,
			"topic": "velux"
		},
		"velux": {
			"host": "klf200.local",
			"password": "velux-secret"
		},
		"homeassistant": {
			"prefix": "homeassistant",
			"invert-awning": false
		},
		"restart": {
			"restart-interval": 24,
			"health-check-interval": 300,
			"restart-on-error": true
		},
		"loglevel": "info"
	}`

	c, err := LoadConfig(writeConfig(t, body))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if c.MQTT.URL != "tcp://mqtt.local:1883" {
		t.Errorf("MQTT.URL = %q", c.MQTT.URL)
	}
	if c.MQTT.Username != "user" {
		t.Errorf("MQTT.Username = %q", c.MQTT.Username)
	}
	if c.MQTT.Password != "pass" {
		t.Errorf("MQTT.Password = %q", c.MQTT.Password)
	}
	if c.MQTT.ClientID != "velux-gw" {
		t.Errorf("MQTT.ClientID = %q", c.MQTT.ClientID)
	}
	if c.MQTT.Topic != "velux" {
		t.Errorf("MQTT.Topic = %q", c.MQTT.Topic)
	}
	if c.Velux.Host != "klf200.local" {
		t.Errorf("Velux.Host = %q", c.Velux.Host)
	}
	if c.Velux.Password != "velux-secret" {
		t.Errorf("Velux.Password = %q", c.Velux.Password)
	}
	if c.HomeAssistant.Prefix != "homeassistant" {
		t.Errorf("HomeAssistant.Prefix = %q", c.HomeAssistant.Prefix)
	}
	if c.HomeAssistant.InvertAwning {
		t.Error("InvertAwning should be false")
	}
	if c.Restart.RestartInterval != 24 {
		t.Errorf("RestartInterval = %d", c.Restart.RestartInterval)
	}
	if c.Restart.HealthCheckInterval != 300 {
		t.Errorf("HealthCheckInterval = %d", c.Restart.HealthCheckInterval)
	}
	if !c.Restart.RestartOnError {
		t.Error("RestartOnError should be true")
	}
	if c.LogLevel != "info" {
		t.Errorf("LogLevel = %q", c.LogLevel)
	}

	// Verify Get() returns same config.
	got := Get()
	if got.MQTT.URL != c.MQTT.URL {
		t.Errorf("Get().MQTT.URL = %q, want %q", got.MQTT.URL, c.MQTT.URL)
	}
}

// ---- ToGatewayConfig ----

func TestToGatewayConfig(t *testing.T) {
	m := MQTTConfig{
		URL:      "tcp://broker:1883",
		Username: "u",
		Password: "p",
		Retain:   true,
		QoS:      1,
		Topic:    "velux",
	}
	gw := m.ToGatewayConfig()
	if gw.URL != m.URL {
		t.Errorf("URL = %q, want %q", gw.URL, m.URL)
	}
	if gw.Username != m.Username {
		t.Errorf("Username = %q", gw.Username)
	}
	if gw.Retain != m.Retain {
		t.Errorf("Retain = %v", gw.Retain)
	}
	if gw.QoS != m.QoS {
		t.Errorf("QoS = %d", gw.QoS)
	}
}

// TestApplyDefaults_PowerCycle pins the power-cycle defaults. Payload and dwell
// defaults apply even when the feature is off, so enabling it later only needs
// the topic.
func TestApplyDefaults_PowerCycle(t *testing.T) {
	c := Config{}
	ApplyDefaults(&c)

	if c.Restart.PowerCycleTopic != "" {
		t.Errorf("PowerCycleTopic = %q, want empty (feature must be opt-in)", c.Restart.PowerCycleTopic)
	}
	if c.Restart.PowerCycleOffPayload != "OFF" {
		t.Errorf("PowerCycleOffPayload = %q, want OFF", c.Restart.PowerCycleOffPayload)
	}
	if c.Restart.PowerCycleOnPayload != "ON" {
		t.Errorf("PowerCycleOnPayload = %q, want ON", c.Restart.PowerCycleOnPayload)
	}
	if c.Restart.PowerCycleOffSeconds != 10 {
		t.Errorf("PowerCycleOffSeconds = %d, want 10", c.Restart.PowerCycleOffSeconds)
	}
}

// TestApplyDefaults_PowerCycleKeepsExplicit makes sure a deliberately short or
// long dwell survives ApplyDefaults.
func TestApplyDefaults_PowerCycleKeepsExplicit(t *testing.T) {
	c := Config{}
	c.Restart.PowerCycleTopic = "cmnd/velux-plug/POWER"
	c.Restart.PowerCycleOffPayload = "0"
	c.Restart.PowerCycleOnPayload = "1"
	c.Restart.PowerCycleOffSeconds = 30
	ApplyDefaults(&c)

	if c.Restart.PowerCycleOffPayload != "0" || c.Restart.PowerCycleOnPayload != "1" {
		t.Errorf("payloads overwritten: %q/%q", c.Restart.PowerCycleOffPayload, c.Restart.PowerCycleOnPayload)
	}
	if c.Restart.PowerCycleOffSeconds != 30 {
		t.Errorf("PowerCycleOffSeconds = %d, want 30", c.Restart.PowerCycleOffSeconds)
	}
}

// TestPowerCycleEnabled is the feature gate: the topic alone decides.
func TestPowerCycleEnabled(t *testing.T) {
	tests := []struct {
		name  string
		topic string
		want  bool
	}{
		{"unset", "", false},
		{"set", "hs100/velux_reset/set", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := RestartConfig{PowerCycleTopic: tt.topic}
			if got := r.PowerCycleEnabled(); got != tt.want {
				t.Errorf("PowerCycleEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPeriodicPowerCycleEnabled: the hard periodic restart needs both the flag
// and a plug topic — the flag alone cannot be honoured.
func TestPeriodicPowerCycleEnabled(t *testing.T) {
	tests := []struct {
		name  string
		flag  bool
		topic string
		want  bool
	}{
		{"neither", false, "", false},
		{"flag only", true, "", false},
		{"topic only", false, "plug/set", false},
		{"both", true, "plug/set", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := RestartConfig{PeriodicPowerCycle: tt.flag, PowerCycleTopic: tt.topic}
			if got := r.PeriodicPowerCycleEnabled(); got != tt.want {
				t.Errorf("PeriodicPowerCycleEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRestartTimeOfDay covers parsing and the reason validation exists: a typo in
// the schedule must fail at startup, not silently disable maintenance.
func TestRestartTimeOfDay(t *testing.T) {
	tests := []struct {
		name    string
		at      string
		wantH   int
		wantM   int
		wantErr bool
	}{
		{"unset", "", 0, 0, false},
		{"midnight", "00:00", 0, 0, false},
		{"two am", "02:00", 2, 0, false},
		{"half past", "23:30", 23, 30, false},
		{"12h form rejected", "2:00 AM", 0, 0, true},
		{"nonsense", "nachts", 0, 0, true},
		{"out of range", "25:00", 0, 0, true},
		{"seconds not accepted", "02:00:00", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, m, err := RestartConfig{RestartAt: tt.at}.RestartTimeOfDay()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if err == nil && (h != tt.wantH || m != tt.wantM) {
				t.Errorf("= %d:%02d, want %d:%02d", h, m, tt.wantH, tt.wantM)
			}
		})
	}
}
