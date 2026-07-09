package main

import (
	"context"
	_ "expvar"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/mqtt-home/velux-mqtt-gw/bridge"
	"github.com/mqtt-home/velux-mqtt-gw/config"
	"github.com/mqtt-home/velux-mqtt-gw/klf200"
	"github.com/mqtt-home/velux-mqtt-gw/version"
	"github.com/philipparndt/go-logger"
	"github.com/philipparndt/mqtt-gateway/mqtt"
)

// pprofAddr is the bind address for the diagnostic listener, matching the miele
// bridge (which always exposes :6060). Empty disables it.
const pprofAddr = ":6060"

// mqttClientIDPrefix is the default client-id prefix used when the config does
// not override MQTT.ClientID. Mirrors the Python bridge's "vlxmqttha" app name.
const mqttClientIDPrefix = "vlxmqttha"

func initPprof() {
	if pprofAddr == "" {
		return
	}
	go func() {
		if err := http.ListenAndServe(pprofAddr, nil); err != nil {
			logger.Error("pprof listener failed", "error", err)
		}
	}()
}

func main() {
	logger.Init("info", logger.Logger())
	logger.Info("velux-mqtt-gw",
		"version", version.Version,
		"commit", version.GitCommit,
		"built", version.BuildTime,
	)

	if len(os.Args) != 2 {
		logger.Error("Expected config file as argument.")
		os.Exit(1)
	}
	configFile := os.Args[1]
	logger.Info("Loading config", "file", configFile)

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		logger.Error("Failed to load config", "error", err)
		os.Exit(1)
	}
	logger.SetLevel(cfg.LogLevel)

	initPprof()

	// Connect MQTT first so the bridge's discovery/state publishes (issued during
	// App.Start) land on a live connection. mqtt-gateway manages retry/backoff and
	// blocks until the initial connection succeeds; it also owns the bridge/state
	// last-will (online/offline), so HA sees the bridge disconnect on an unclean
	// exit. The client-id prefix falls back to the config override when set.
	clientID := mqttClientIDPrefix
	if cfg.MQTT.ClientID != "" {
		clientID = cfg.MQTT.ClientID
	}
	mqtt.Start(cfg.MQTT.ToGatewayConfig(), clientID)

	// Build the KLF200 client and bridge manager, then wire the App.
	client := klf200.NewClient(cfg.Velux.Host, cfg.Velux.Password)
	mqttWrapper := bridge.NewMQTT(cfg)
	mgr := bridge.NewManager(cfg, client, mqttWrapper)
	app := newApp(cfg, client, mgr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.Start(ctx); err != nil {
		logger.Error("Startup failed", "error", err)
		// Startup failed after a (possibly partial) KLF200 connect. Attempt a
		// clean disconnect so we never leave a zombie session slot behind, then
		// exit non-zero.
		app.Stop()
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down")
	cancel()
	app.Stop()
	// MQTT teardown (bridge/state -> offline via last-will) happens on process
	// exit; mqtt-gateway exposes no explicit stop, matching the miele bridge.
}
