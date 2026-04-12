// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/thepixelabs/kubecat/internal/history"
	"github.com/thepixelabs/kubecat/internal/storage"
)

// TimelineEvent is a JSON-friendly timeline event.
type TimelineEvent struct {
	ID              int64  `json:"id"`
	Cluster         string `json:"cluster"`
	Namespace       string `json:"namespace"`
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Reason          string `json:"reason"`
	Message         string `json:"message"`
	FirstSeen       string `json:"firstSeen"`
	LastSeen        string `json:"lastSeen"`
	Count           int    `json:"count"`
	SourceComponent string `json:"sourceComponent"`
}

// TimelineFilter is used to filter timeline events.
type TimelineFilter struct {
	Namespace    string `json:"namespace"`
	Kind         string `json:"kind"`
	Type         string `json:"type"`
	SinceMinutes int    `json:"sinceMinutes"`
	Limit        int    `json:"limit"`
}

// SnapshotInfo contains snapshot metadata.
type SnapshotInfo struct {
	Timestamp string `json:"timestamp"`
}

// SnapshotDiffResult contains the diff between two snapshots.
type SnapshotDiffResult struct {
	Before   string           `json:"before"`
	After    string           `json:"after"`
	Added    []ResourceChange `json:"added"`
	Removed  []ResourceChange `json:"removed"`
	Modified []ResourceChange `json:"modified"`
}

// ResourceChange represents a changed resource.
type ResourceChange struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	OldStatus string `json:"oldStatus,omitempty"`
	NewStatus string `json:"newStatus,omitempty"`
}

// GetTimelineEvents returns events from the history database.
func (a *App) GetTimelineEvents(filter TimelineFilter) ([]TimelineEvent, error) {
	if a.eventCollector == nil {
		return nil, fmt.Errorf("timeline not available: history database not initialized")
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Build storage filter
	storageFilter := storage.EventFilter{
		Cluster:   a.nexus.Clusters.ActiveContext(),
		Namespace: filter.Namespace,
		Kind:      filter.Kind,
		Type:      filter.Type,
		Limit:     filter.Limit,
	}

	if storageFilter.Limit == 0 {
		storageFilter.Limit = 500
	}

	if filter.SinceMinutes > 0 {
		storageFilter.Since = time.Now().UTC().Add(-time.Duration(filter.SinceMinutes) * time.Minute)
	} else {
		storageFilter.Since = time.Now().UTC().Add(-24 * time.Hour) // Default to last 24 hours
	}

	events, err := a.eventCollector.GetEvents(ctx, storageFilter)
	if err != nil {
		return nil, err
	}

	result := make([]TimelineEvent, len(events))
	for i, e := range events {
		result[i] = TimelineEvent{
			ID:              e.ID,
			Cluster:         e.Cluster,
			Namespace:       e.Namespace,
			Kind:            e.Kind,
			Name:            e.Name,
			Type:            e.Type,
			Reason:          e.Reason,
			Message:         e.Message,
			FirstSeen:       e.FirstSeen.Format(time.RFC3339),
			LastSeen:        e.LastSeen.Format(time.RFC3339),
			Count:           e.Count,
			SourceComponent: e.SourceComponent,
		}
	}

	return result, nil
}

// GetSnapshots returns available snapshot timestamps.
func (a *App) GetSnapshots(limit int) ([]SnapshotInfo, error) {
	if a.snapshotter == nil {
		return nil, fmt.Errorf("snapshots not available: history database not initialized")
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	cluster := a.nexus.Clusters.ActiveContext()
	if cluster == "" {
		return nil, fmt.Errorf("no active cluster")
	}

	if limit == 0 {
		limit = 50
	}

	timestamps, err := a.snapshotter.ListSnapshots(ctx, cluster, limit)
	if err != nil {
		return nil, err
	}

	result := make([]SnapshotInfo, len(timestamps))
	for i, ts := range timestamps {
		result[i] = SnapshotInfo{
			Timestamp: ts.Format(time.RFC3339),
		}
	}

	return result, nil
}

// GetSnapshotDiff compares two snapshots and returns the differences.
func (a *App) GetSnapshotDiff(beforeTimestamp, afterTimestamp string) (*SnapshotDiffResult, error) {
	if a.snapshotter == nil {
		return nil, fmt.Errorf("snapshots not available: history database not initialized")
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	cluster := a.nexus.Clusters.ActiveContext()
	if cluster == "" {
		return nil, fmt.Errorf("no active cluster")
	}

	beforeTime, err := time.Parse(time.RFC3339, beforeTimestamp)
	if err != nil {
		return nil, fmt.Errorf("invalid before timestamp: %w", err)
	}

	afterTime, err := time.Parse(time.RFC3339, afterTimestamp)
	if err != nil {
		return nil, fmt.Errorf("invalid after timestamp: %w", err)
	}

	before, err := a.snapshotter.GetSnapshot(ctx, cluster, beforeTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get before snapshot: %w", err)
	}

	after, err := a.snapshotter.GetSnapshot(ctx, cluster, afterTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get after snapshot: %w", err)
	}

	diff := history.CompareSnapshots(before, after)

	result := &SnapshotDiffResult{
		Before:   diff.Before.Format(time.RFC3339),
		After:    diff.After.Format(time.RFC3339),
		Added:    make([]ResourceChange, len(diff.Added)),
		Removed:  make([]ResourceChange, len(diff.Removed)),
		Modified: make([]ResourceChange, len(diff.Modified)),
	}

	for i, c := range diff.Added {
		result.Added[i] = ResourceChange{Kind: c.Kind, Name: c.Name, Namespace: c.Namespace}
	}
	for i, c := range diff.Removed {
		result.Removed[i] = ResourceChange{Kind: c.Kind, Name: c.Name, Namespace: c.Namespace}
	}
	for i, c := range diff.Modified {
		result.Modified[i] = ResourceChange{
			Kind: c.Kind, Name: c.Name, Namespace: c.Namespace,
			OldStatus: c.OldStatus, NewStatus: c.NewStatus,
		}
	}

	return result, nil
}

// TakeSnapshot takes a manual snapshot of the current cluster state.
func (a *App) TakeSnapshot() error {
	if a.snapshotter == nil {
		return fmt.Errorf("snapshots not available: history database not initialized")
	}

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	return a.snapshotter.TakeManualSnapshot(ctx)
}

// IsTimelineAvailable returns true if the timeline feature is available.
func (a *App) IsTimelineAvailable() bool {
	available := a.eventCollector != nil && a.db != nil
	return available
}
