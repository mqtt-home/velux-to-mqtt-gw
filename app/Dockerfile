# syntax=docker/dockerfile:1.24

# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X 'github.com/mqtt-home/velux-to-mqtt-gw/version.Version=${VERSION}' -X 'github.com/mqtt-home/velux-to-mqtt-gw/version.GitCommit=${GIT_COMMIT}' -X 'github.com/mqtt-home/velux-to-mqtt-gw/version.BuildTime=${BUILD_TIME}'" \
    -trimpath -o velux-to-mqtt-gw .

# Final stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /app/velux-to-mqtt-gw /velux-to-mqtt-gw
USER nonroot:nonroot

ENTRYPOINT ["/velux-to-mqtt-gw", "/var/lib/velux-to-mqtt-gw/config.json"]
