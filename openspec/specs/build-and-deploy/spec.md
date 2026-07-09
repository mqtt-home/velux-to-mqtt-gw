# build-and-deploy Specification

## Purpose
TBD - created by archiving change rewrite-in-go. Update Purpose after archive.
## Requirements
### Requirement: Makefile-driven workflow

The system SHALL provide a Makefile with targets to build, test, lint, run (with a config path),
build a Docker image, and clean, mirroring the sibling `miele-to-mqtt-gw` project. The build
SHALL produce a single static binary with size-optimizing link flags.

#### Scenario: Build target

- **WHEN** `make build` is run
- **THEN** a single static binary is produced with `-ldflags="-s -w" -trimpath`

#### Scenario: Test and lint targets

- **WHEN** `make test` and `make lint` are run
- **THEN** the Go tests execute and the configured linter runs

### Requirement: Distroless container image

The system SHALL provide a multi-stage Dockerfile that compiles the Go binary and packages it on
a distroless base image, with the binary as entrypoint and the config path passed as an argument.

#### Scenario: Image build

- **WHEN** the Docker image is built
- **THEN** the final image is distroless, contains only the static binary, and runs it as
  entrypoint

### Requirement: Release configuration

The system SHALL provide a goreleaser configuration producing multi-architecture binaries and
container images, mirroring the sibling project.

#### Scenario: Release build

- **WHEN** a release is produced via goreleaser
- **THEN** multi-architecture artifacts and container images are generated

