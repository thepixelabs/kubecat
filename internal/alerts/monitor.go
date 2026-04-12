// Package alerts monitors clusters for anomalous conditions and emits
// proactive AI query suggestions to the frontend.
package alerts

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
	"github.com/thepixelabs/kubecat/internal/events"
)

const (
	scanInterval   = 60 * time.Second
	cooldown       = 30 * time.Minute
	clusterTimeout = 30 * time.Second
)

// Alert is the payload emitted on the "ai:alert" Wails event.
type Alert struct {
	Cluster        string        `json:"cluster"`
	Namespace      string        `json:"namespace"`
	Kind           string        `json:"kind"`
	Name           string        `json:"name"`
	Message        string        `json:"message"`
	Severity       string        `json:"severity"` // "warning" | "critical"
	SuggestedQuery string        `json:"suggestedQuery"`
	ContextItems   []ContextItem `json:"contextItems,omitempty"`
}

// ContextItem is a resource reference to send alongside the suggested query.
type ContextItem struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// dedupKey uniquely identifies an alert for cooldown deduplication.
type dedupKey struct {
	cluster   string
	namespace string
	kind      string
	name      string
	message   string
}

// AlertMonitor periodically scans all active clusters for alert conditions.
type AlertMonitor struct {
	manager client.Manager
	emitter events.EmitterInterface

	mu       sync.Mutex
	lastSeen map[dedupKey]time.Time
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewAlertMonitor creates an AlertMonitor. Call Start to begin scanning.
func NewAlertMonitor(mgr client.Manager, em events.EmitterInterface) *AlertMonitor {
	return &AlertMonitor{
		manager:  mgr,
		emitter:  em,
		lastSeen: make(map[dedupKey]time.Time),
		done:     make(chan struct{}),
	}
}

// Start begins the periodic scan loop.
func (m *AlertMonitor) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	go m.loop(ctx)
}

// Stop shuts down the scan loop.
func (m *AlertMonitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	<-m.done
}

func (m *AlertMonitor) loop(ctx context.Context) {
	defer close(m.done)

	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	// Initial scan.
	m.scan(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.scan(ctx)
		}
	}
}

// scan checks all active clusters and emits alerts for detected issues.
func (m *AlertMonitor) scan(ctx context.Context) {
	clusters := m.manager.List()
	for _, info := range clusters {
		if info.Status != client.StatusConnected {
			continue
		}
		cl, err := m.manager.Get(info.Context)
		if err != nil {
			continue
		}

		clusterCtx, cancel := context.WithTimeout(ctx, clusterTimeout)
		m.scanCluster(clusterCtx, cl, info.Context)
		cancel()
	}
}

// scanCluster checks a single cluster for alert conditions.
func (m *AlertMonitor) scanCluster(ctx context.Context, cl client.ClusterClient, clusterName string) {
	// Check for crash-looping pods.
	m.checkCrashLoopingPods(ctx, cl, clusterName, "")

	// Check for failed pods.
	m.checkFailedPods(ctx, cl, clusterName, "")

	// Check for pending PVCs.
	m.checkPendingPVCs(ctx, cl, clusterName, "")
}

func (m *AlertMonitor) checkCrashLoopingPods(ctx context.Context, cl client.ClusterClient, cluster, namespace string) {
	list, err := cl.List(ctx, "pods", client.ListOptions{Namespace: namespace, Limit: 200})
	if err != nil {
		return
	}

	for _, r := range list.Items {
		if r.Status == "CrashLoopBackOff" || r.Status == "Error" {
			m.maybeEmit(Alert{
				Cluster:        cluster,
				Namespace:      r.Namespace,
				Kind:           "Pod",
				Name:           r.Name,
				Message:        fmt.Sprintf("Pod %s/%s is in %s state", r.Namespace, r.Name, r.Status),
				Severity:       "critical",
				SuggestedQuery: fmt.Sprintf("Why is pod %s in %s? What are the recent logs and events?", r.Name, r.Status),
				ContextItems: []ContextItem{
					{Kind: "Pod", Namespace: r.Namespace, Name: r.Name},
				},
			})
		}
	}
}

func (m *AlertMonitor) checkFailedPods(ctx context.Context, cl client.ClusterClient, cluster, namespace string) {
	list, err := cl.List(ctx, "pods", client.ListOptions{Namespace: namespace, Limit: 200})
	if err != nil {
		return
	}

	for _, r := range list.Items {
		if r.Status == "Failed" || r.Status == "OOMKilled" {
			m.maybeEmit(Alert{
				Cluster:        cluster,
				Namespace:      r.Namespace,
				Kind:           "Pod",
				Name:           r.Name,
				Message:        fmt.Sprintf("Pod %s/%s has failed with status %s", r.Namespace, r.Name, r.Status),
				Severity:       "warning",
				SuggestedQuery: fmt.Sprintf("What caused pod %s to fail with status %s?", r.Name, r.Status),
				ContextItems: []ContextItem{
					{Kind: "Pod", Namespace: r.Namespace, Name: r.Name},
				},
			})
		}
	}
}

func (m *AlertMonitor) checkPendingPVCs(ctx context.Context, cl client.ClusterClient, cluster, namespace string) {
	list, err := cl.List(ctx, "persistentvolumeclaims", client.ListOptions{Namespace: namespace, Limit: 100})
	if err != nil {
		return
	}

	for _, r := range list.Items {
		if r.Status == "Pending" {
			m.maybeEmit(Alert{
				Cluster:        cluster,
				Namespace:      r.Namespace,
				Kind:           "PersistentVolumeClaim",
				Name:           r.Name,
				Message:        fmt.Sprintf("PVC %s/%s has been pending", r.Namespace, r.Name),
				Severity:       "warning",
				SuggestedQuery: fmt.Sprintf("Why is PVC %s still pending? Is there a matching PersistentVolume?", r.Name),
				ContextItems: []ContextItem{
					{Kind: "PersistentVolumeClaim", Namespace: r.Namespace, Name: r.Name},
				},
			})
		}
	}
}

// maybeEmit emits an alert if it is not within the cooldown window.
func (m *AlertMonitor) maybeEmit(alert Alert) {
	key := dedupKey{
		cluster:   alert.Cluster,
		namespace: alert.Namespace,
		kind:      alert.Kind,
		name:      alert.Name,
		message:   alert.Message,
	}

	m.mu.Lock()
	last, exists := m.lastSeen[key]
	if exists && time.Since(last) < cooldown {
		m.mu.Unlock()
		return
	}
	m.lastSeen[key] = time.Now()
	m.mu.Unlock()

	slog.Info("alert monitor: emitting alert",
		slog.String("cluster", alert.Cluster),
		slog.String("kind", alert.Kind),
		slog.String("name", alert.Name))

	m.emitter.Emit("ai:alert", alert)
}
