## MODIFIED Requirements

### Requirement: Configuration schema and defaults

The system SHALL accept the sections `mqtt` (url, optional username/password, retain), `velux`
(host, password), `homeassistant` (optional prefix, invert-awning), `restart` (restart-interval,
health-check-interval, restart-on-error, and the optional power-cycle settings
`power-cycle-topic`, `power-cycle-off-payload`, `power-cycle-on-payload`, `power-cycle-off-seconds`),
and `loglevel`, applying documented defaults for unset optional fields. The power-cycle feature
SHALL be disabled unless `power-cycle-topic` is set to a non-empty value.

#### Scenario: Defaults applied

- **WHEN** optional fields are omitted
- **THEN** defaults are applied (e.g. retain=true, prefix="", invert-awning=false, loglevel=info,
  the restart timers default to the Python values, and the power-cycle feature is disabled with
  `power-cycle-off-payload`="OFF", `power-cycle-on-payload`="ON", `power-cycle-off-seconds`=10)

#### Scenario: Explicit values honored

- **WHEN** an optional field is set explicitly
- **THEN** the explicit value overrides the default

#### Scenario: Power-cycle enabled by topic

- **WHEN** `restart.power-cycle-topic` is set to a non-empty string
- **THEN** the automated hardware power-cycle recovery feature is enabled using the configured (or
  defaulted) payloads and dwell

#### Scenario: Power-cycle disabled by default

- **WHEN** `restart.power-cycle-topic` is absent or empty
- **THEN** the power-cycle feature is disabled and no power-cycle behavior is active
