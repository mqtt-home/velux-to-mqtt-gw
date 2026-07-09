## ADDED Requirements

### Requirement: JSON configuration with environment substitution

The system SHALL load configuration from a JSON file whose path is given as the single
command-line argument, substituting `${NAME}` occurrences with the corresponding environment
variable value (empty string when unset) before parsing.

#### Scenario: Env-var substitution

- **WHEN** the config contains `${MQTT_PASSWORD}` and that environment variable is set
- **THEN** the parsed config uses the environment variable's value

#### Scenario: Missing env var

- **WHEN** the config references an undefined environment variable
- **THEN** it is substituted with an empty string and parsing continues

### Requirement: Configuration schema and defaults

The system SHALL accept the sections `mqtt` (url, optional username/password, retain), `velux`
(host, password), `homeassistant` (optional prefix, invert-awning), `restart` (restart-interval,
health-check-interval, restart-on-error), and `loglevel`, applying documented defaults for unset
optional fields.

#### Scenario: Defaults applied

- **WHEN** optional fields are omitted
- **THEN** defaults are applied (e.g. retain=true, prefix="", invert-awning=false, loglevel=info,
  and the restart timers default to the Python values)

#### Scenario: Explicit values honored

- **WHEN** an optional field is set explicitly
- **THEN** the explicit value overrides the default

### Requirement: Required-field validation

The system SHALL fail to start with a clear error when a required field is missing, specifically
`mqtt.url`, `velux.host`, and `velux.password`.

#### Scenario: Missing required field

- **WHEN** `velux.password` is absent
- **THEN** configuration loading returns an error naming the missing field and the application
  exits non-zero
