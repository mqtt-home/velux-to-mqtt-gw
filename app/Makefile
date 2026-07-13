GO = go
GOFLAGS = -v

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')

MODULE = github.com/mqtt-home/velux-to-mqtt-gw
LDFLAGS = -ldflags "-s -w -X '$(MODULE)/version.Version=$(VERSION)' -X '$(MODULE)/version.GitCommit=$(GIT_COMMIT)' -X '$(MODULE)/version.BuildTime=$(BUILD_TIME)'"

BUILD_DIR = build
BINARY_NAME = velux-to-mqtt-gw
CONFIG ?= $(PWD)/../production/config/config.json

DOCKER_IMAGE_NAME = ghcr.io/mqtt-home/velux-to-mqtt-gw:dev

.PHONY: build test coverage vet lint run image clean deps version release-check release-snapshot release-next release

build:
	@echo "Building $(BINARY_NAME)..."
	@$(GO) build $(GOFLAGS) $(LDFLAGS) -trimpath -o $(BUILD_DIR)/$(BINARY_NAME) .

test:
	@echo "Running tests..."
	@$(GO) test ./... -race -cover

coverage:
	@echo "Running tests with coverage..."
	@$(GO) test -coverprofile=coverage.out ./...
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

vet:
	@$(GO) vet ./...

lint: vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Running golangci-lint..."; \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, skipping (go vet already ran)."; \
	fi

run: build
	@echo "Running $(BINARY_NAME) with $(CONFIG)..."
	@$(BUILD_DIR)/$(BINARY_NAME) $(CONFIG)

image:
	@echo "Building Docker image (amd64)..."
	@docker buildx build --platform=linux/amd64 --load -f Dockerfile.goreleaser -t $(DOCKER_IMAGE_NAME) . \
		|| docker build -f Dockerfile.goreleaser -t $(DOCKER_IMAGE_NAME) .

clean:
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

deps:
	@$(GO) mod download
	@$(GO) mod tidy

version:
	@echo "Version:    $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

release-check:
	@goreleaser check

release-snapshot:
	@echo "Building local snapshot (no publish)..."
	@goreleaser release --snapshot --clean --skip=publish

# Preview the next version computed from Conventional Commits since the
# last tag (feat: → minor, fix: → patch, BREAKING CHANGE / feat!: → major).
release-next:
	@command -v svu >/dev/null 2>&1 || { \
		echo "svu not installed. Install with: go install github.com/caarlos0/svu@latest"; \
		exit 1; \
	}
	@svu next

# Trigger a real release: compute the next version from Conventional Commits,
# tag HEAD, and push the tag. The tag push fires publish_docker.yml (multi-arch
# GHCR image). Override the computed version with RELEASE_VERSION=vX.Y.Z if you
# really need a manual bump.
release:
	@if [ -n "$(RELEASE_VERSION)" ]; then \
		NEXT="$(RELEASE_VERSION)"; \
	else \
		command -v svu >/dev/null 2>&1 || { \
			echo "svu not installed. Install with: go install github.com/caarlos0/svu@latest"; \
			exit 1; \
		}; \
		NEXT=$$(svu next); \
	fi; \
	case "$$NEXT" in v[0-9]*.[0-9]*.[0-9]*) ;; \
		*) echo "Version '$$NEXT' does not match vX.Y.Z"; exit 1;; \
	esac; \
	if git rev-parse "$$NEXT" >/dev/null 2>&1; then \
		echo "Tag $$NEXT already exists"; exit 1; \
	fi; \
	echo "Tagging $$NEXT and pushing..."; \
	git tag -a "$$NEXT" -m "Release $$NEXT" && \
	git push origin "$$NEXT"
