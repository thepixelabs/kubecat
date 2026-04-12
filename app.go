package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/thepixelabs/kubecat/internal/audit"
	"github.com/thepixelabs/kubecat/internal/config"
	"github.com/thepixelabs/kubecat/internal/core"
	"github.com/thepixelabs/kubecat/internal/events"
	"github.com/thepixelabs/kubecat/internal/health"
	"github.com/thepixelabs/kubecat/internal/history"
	"github.com/thepixelabs/kubecat/internal/keychain"
	"github.com/thepixelabs/kubecat/internal/storage"
	"github.com/thepixelabs/kubecat/internal/terminal"
	"github.com/thepixelabs/kubecat/internal/updater"
)

// agentSession holds runtime state for a running AIAgentQuery session.
type agentSession struct {
	// cancel stops the agent loop when called.
	cancel context.CancelFunc
	// approvalCh receives approval decisions keyed by toolCallID.
	// The agent blocks reading from this channel while waiting for the user.
	approvalCh chan agentApprovalMsg
}

// agentApprovalMsg carries a user's approve/reject decision for a pending tool call.
type agentApprovalMsg struct {
	toolCallID string
	approved   bool
}

// App is the bridge between the frontend and the core Kubecat services.
type App struct {
	ctx             context.Context
	nexus           *core.Kubecat
	db              *storage.DB
	eventCollector  *history.EventCollector
	snapshotter     *history.Snapshotter
	terminalManager *terminal.Manager
	emitter         *events.Emitter
	healthMonitor   *health.ClusterHealthMonitor
	retentionMgr    *storage.RetentionManager
	updateChecker   *updater.Updater

	// Resource watcher state (used by resource_watcher.go).
	mu       sync.Mutex
	watchers map[string]context.CancelFunc

	// Agent session registry (used by app_agent.go).
	agentMu       sync.Mutex
	agentSessions map[string]*agentSession
}

// NewApp creates a new App bridge.
func NewApp(
	nexus *core.Kubecat,
	db *storage.DB,
	eventCollector *history.EventCollector,
	snapshotter *history.Snapshotter,
	emitter *events.Emitter,
	healthMonitor *health.ClusterHealthMonitor,
	retentionMgr *storage.RetentionManager,
) *App {
	return &App{
		nexus:           nexus,
		db:              db,
		eventCollector:  eventCollector,
		snapshotter:     snapshotter,
		terminalManager: terminal.NewManager(),
		emitter:         emitter,
		healthMonitor:   healthMonitor,
		retentionMgr:    retentionMgr,
		watchers:        make(map[string]context.CancelFunc),
		agentSessions:   make(map[string]*agentSession),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.terminalManager.SetContext(ctx)
	if a.emitter != nil {
		a.emitter.SetContext(ctx)
	}
	if a.healthMonitor != nil {
		a.healthMonitor.Start(ctx)
	}
	if a.retentionMgr != nil {
		a.retentionMgr.Start(ctx)
	}

	// Initialize the audit logger early so all subsequent operations are captured.
	if err := audit.Init(); err != nil {
		slog.Warn("audit logger init failed — sensitive operations will not be audited",
			slog.Any("error", err))
	}

	// Migrate any plaintext API keys from config.yaml into the OS keychain.
	// Best-effort: failures are logged inside MigrateAPIKeys, never abort startup.
	keychain.MigrateAPIKeys()

	// Start the update checker if enabled in config (default: false, opt-in).
	// Set checkForUpdates: true in config.yaml to enable the background goroutine.
	if updateCfg, updateCfgErr := config.Load(); updateCfgErr == nil && updateCfg.Kubecat.CheckForUpdates && a.emitter != nil {
		a.updateChecker = updater.New(a.emitter)
		a.updateChecker.Start(ctx)
		slog.Debug("update checker started")
	}
}

// shutdown is called when the app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.updateChecker != nil {
		a.updateChecker.Stop()
	}
	if a.healthMonitor != nil {
		a.healthMonitor.Stop()
	}
	if a.retentionMgr != nil {
		a.retentionMgr.Stop()
	}
	if a.eventCollector != nil {
		a.eventCollector.Stop()
	}
	if a.snapshotter != nil {
		a.snapshotter.Stop()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
	_ = a.nexus.Close()
	audit.Shutdown()
}

// checkReadOnly returns an error if the application is configured in read-only
// mode. Call this at the top of every method that mutates cluster state or
// spawns an interactive session that could mutate cluster state.
func (a *App) checkReadOnly() error {
	cfg, err := config.Load()
	if err != nil {
		// If we cannot read the config we cannot verify safety — fail closed.
		return fmt.Errorf("read-only check failed: unable to load config: %w", err)
	}
	if cfg.Kubecat.ReadOnly {
		return fmt.Errorf("operation blocked: Kubecat is running in read-only mode (readOnly: true in config)")
	}
	return nil
}
