// Package main is the entry point for the Kubecat GUI application.
package main

import (
	"embed"
	"log"
	"log/slog"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/thepixelabs/kubecat/internal/config"
	"github.com/thepixelabs/kubecat/internal/core"
	"github.com/thepixelabs/kubecat/internal/events"
	"github.com/thepixelabs/kubecat/internal/health"
	"github.com/thepixelabs/kubecat/internal/history"
	"github.com/thepixelabs/kubecat/internal/logging"
	"github.com/thepixelabs/kubecat/internal/storage"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Initialize structured logging as the very first thing so all subsequent
	// code (including service startup) uses the configured logger.
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		// We cannot log via slog yet — fall back to stdlib log.
		log.Printf("Warning: failed to load config, using defaults: %v", cfgErr)
		cfg = config.Default()
	}

	logPath := filepath.Join(config.StateDir(), "kubecat.log")
	logCloser, logErr := logging.Setup(logPath, cfg.Kubecat.Logger.LogLevel)
	if logErr != nil {
		log.Printf("Warning: structured logging unavailable, falling back to stderr: %v", logErr)
	} else {
		defer logCloser.Close()
	}

	slog.Info("kubecat starting")

	// Create the core Kubecat services
	kubecat := core.New()

	// Open the history database
	db, err := storage.Open()
	if err != nil {
		slog.Warn("failed to open history database; timeline features disabled", slog.Any("error", err))
	}

	// Observability wiring: emitter must be constructed first so history
	// services can inject it for reactive push events.
	emitter := &events.Emitter{}

	// Create history services if DB is available
	var eventCollector *history.EventCollector
	var snapshotter *history.Snapshotter
	if db != nil && kubecat.Clusters.Manager() != nil {
		eventCollector = history.NewEventCollector(db, kubecat.Clusters.Manager(), history.DefaultEventCollectorConfig(), emitter)
		snapshotter = history.NewSnapshotter(db, kubecat.Clusters.Manager(), history.DefaultSnapshotterConfig(), emitter)

		// Wire correlator so ingested events get linked into causal chains.
		correlator := history.NewCorrelator(db)
		eventCollector.SetCorrelator(correlator)

		// Start background collection
		eventCollector.Start()
		snapshotter.Start()
	} else if db != nil {
		slog.Warn("history collection disabled: cluster manager not initialized")
	}

	// Health monitor and retention manager (also use emitter / db).
	// All three are no-ops if their dependencies are nil.

	var healthMonitor *health.ClusterHealthMonitor
	if kubecat.Clusters.Manager() != nil {
		healthMonitor = health.NewClusterHealthMonitor(kubecat.Clusters.Manager(), emitter)
	}

	var retentionMgr *storage.RetentionManager
	if db != nil {
		retentionMgr = storage.NewRetentionManager(db, storage.DefaultRetentionConfig())
	}

	// Create the app bridge for the frontend
	app := NewApp(kubecat, db, eventCollector, snapshotter, emitter, healthMonitor, retentionMgr)

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "Kubecat",
		Width:  1400,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "Kubecat",
				Message: "Kubernetes Command Center\n\n© 2026 Kubecat",
			},
		},
	})

	if err != nil {
		slog.Error("wails run failed", slog.Any("error", err))
		log.Fatal(err)
	}
}
