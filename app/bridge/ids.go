package bridge

import "strings"

// umlautReplacer transliterates German umlauts the same way as the Python
// bridge's generate_id (ä→ae, ö→oe, ü→ue, ß→ss). Applied AFTER lowercasing, so
// only lowercase forms need entries — matching the Python translate table
// which ran on an already-lowercased string.
var umlautReplacer = strings.NewReplacer(
	"ä", "ae",
	"ö", "oe",
	"ü", "ue",
	"ß", "ss",
)

// GenerateID derives a cover's stable MQTT id from its KLF200 node name,
// porting VeluxMqttHomeassistant.generate_id exactly:
//
//	"vlx-" + name with spaces→hyphens, lowercased, umlauts transliterated
//
// Order matches Python: replace(" ", "-") then .lower() then translate(umlauts).
// e.g. "Dachfenster Büro" -> "vlx-dachfenster-buero".
func GenerateID(name string) string {
	s := strings.ReplaceAll(name, " ", "-")
	s = strings.ToLower(s)
	s = umlautReplacer.Replace(s)
	return "vlx-" + s
}

// Topics bundles every MQTT topic for a single cover and its keep-open switch.
// The base id is the generated id (e.g. "vlx-dachfenster-buero"); prefix is the
// configured HomeAssistant.Prefix, prepended verbatim as the Python bridge did
// (HA_PREFIX + name / HA_PREFIX + mqttid). The base for entity topics is thus
// "{prefix}{id}", and the keep-open switch hangs off "{prefix}{id}-keepopen".
type Topics struct {
	// Base is "{prefix}{id}", the cover entity's base topic.
	Base string
	// KeepOpenBase is "{prefix}{id}-keepopen", the switch entity's base topic.
	KeepOpenBase string

	// Cover entity topics.
	State     string // {prefix}{id}/state
	Position  string // {prefix}{id}/position
	Available string // {prefix}{id}/available
	Set       string // {prefix}{id}/set  (OPEN/CLOSE/STOP/0-100)

	// Keep-open switch topics.
	KeepOpenState     string // {prefix}{id}-keepopen/state
	KeepOpenAvailable string // {prefix}{id}-keepopen/available
	KeepOpenSet       string // {prefix}{id}-keepopen/set  (ON/OFF)
}

// NewTopics builds the full topic set for a cover, given the HA prefix and the
// generated id. This is the single seam for the topic layout; every publisher
// and subscriber uses these fields rather than re-formatting strings.
func NewTopics(prefix, id string) Topics {
	base := prefix + id
	keepOpenBase := prefix + id + "-keepopen"
	return Topics{
		Base:              base,
		KeepOpenBase:      keepOpenBase,
		State:             base + "/state",
		Position:          base + "/position",
		Available:         base + "/available",
		Set:               base + "/set",
		KeepOpenState:     keepOpenBase + "/state",
		KeepOpenAvailable: keepOpenBase + "/available",
		KeepOpenSet:       keepOpenBase + "/set",
	}
}
