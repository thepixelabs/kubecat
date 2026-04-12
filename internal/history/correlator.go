package history

import (
	"context"
	"strings"
	"time"

	"github.com/thepixelabs/kubecat/internal/storage"
)

// CorrelationRule defines how to link events.
type CorrelationRule struct {
	Name         string
	SourceKind   string
	SourceReason string
	TargetKind   string
	TargetReason string
	TimeWindow   time.Duration
	Confidence   float64
	Relationship string
}

// DefaultCorrelationRules are the built-in correlation rules.
var DefaultCorrelationRules = []CorrelationRule{
	{
		Name:         "configmap-to-pod",
		SourceKind:   "ConfigMap",
		SourceReason: "",
		TargetKind:   "Pod",
		TargetReason: "",
		TimeWindow:   5 * time.Minute,
		Confidence:   0.7,
		Relationship: "config_change_affected",
	},
	{
		Name:         "deployment-scaling",
		SourceKind:   "Deployment",
		SourceReason: "ScalingReplicaSet",
		TargetKind:   "Pod",
		TargetReason: "",
		TimeWindow:   2 * time.Minute,
		Confidence:   0.95,
		Relationship: "scaling_caused",
	},
	{
		Name:         "replicaset-to-pod",
		SourceKind:   "ReplicaSet",
		SourceReason: "",
		TargetKind:   "Pod",
		TargetReason: "",
		TimeWindow:   2 * time.Minute,
		Confidence:   0.9,
		Relationship: "managed_by",
	},
	{
		Name:         "job-to-pod",
		SourceKind:   "Job",
		SourceReason: "",
		TargetKind:   "Pod",
		TargetReason: "",
		TimeWindow:   5 * time.Minute,
		Confidence:   0.9,
		Relationship: "job_created",
	},
	{
		Name:         "secret-to-pod",
		SourceKind:   "Secret",
		SourceReason: "",
		TargetKind:   "Pod",
		TargetReason: "",
		TimeWindow:   5 * time.Minute,
		Confidence:   0.6,
		Relationship: "secret_change_affected",
	},
	{
		Name:         "pvc-to-pod",
		SourceKind:   "PersistentVolumeClaim",
		SourceReason: "",
		TargetKind:   "Pod",
		TargetReason: "",
		TimeWindow:   10 * time.Minute,
		Confidence:   0.8,
		Relationship: "storage_issue",
	},
	{
		Name:         "node-to-pod",
		SourceKind:   "Node",
		SourceReason: "",
		TargetKind:   "Pod",
		TargetReason: "",
		TimeWindow:   10 * time.Minute,
		Confidence:   0.75,
		Relationship: "node_issue_affected",
	},
}

// Correlator links related events.
type Correlator struct {
	db        *storage.DB
	eventRepo *storage.EventRepository
	corrRepo  *storage.CorrelationRepository
	rules     []CorrelationRule
}

// NewCorrelator creates a new correlator.
func NewCorrelator(db *storage.DB) *Correlator {
	return &Correlator{
		db:        db,
		eventRepo: storage.NewEventRepository(db),
		corrRepo:  storage.NewCorrelationRepository(db),
		rules:     DefaultCorrelationRules,
	}
}

// SetRules sets custom correlation rules.
func (c *Correlator) SetRules(rules []CorrelationRule) {
	c.rules = rules
}

// AddRule adds a correlation rule.
func (c *Correlator) AddRule(rule CorrelationRule) {
	c.rules = append(c.rules, rule)
}

// CorrelateEvent finds correlations for a given event.
func (c *Correlator) CorrelateEvent(ctx context.Context, event storage.StoredEvent) ([]storage.Correlation, error) {
	var correlations []storage.Correlation

	for _, rule := range c.rules {
		matches, err := c.findMatchesForRule(ctx, event, rule)
		if err != nil {
			continue
		}

		for _, match := range matches {
			// Calculate dynamic confidence based on timing
			confidence := c.calculateConfidence(event, match, rule)

			corr := storage.Correlation{
				SourceEventID: event.ID,
				TargetEventID: match.ID,
				Confidence:    confidence,
				Relationship:  rule.Relationship,
			}

			// Save the correlation
			if err := c.corrRepo.Save(ctx, &corr); err == nil {
				correlations = append(correlations, corr)
			}
		}
	}

	return correlations, nil
}

// findMatchesForRule finds events matching a correlation rule.
func (c *Correlator) findMatchesForRule(ctx context.Context, source storage.StoredEvent, rule CorrelationRule) ([]storage.StoredEvent, error) {
	// Check if source matches the rule
	if !c.matchesKindReason(source, rule.SourceKind, rule.SourceReason) {
		return nil, nil
	}

	// Find target events within the time window
	filter := storage.EventFilter{
		Cluster:   source.Cluster,
		Namespace: source.Namespace,
		Kind:      rule.TargetKind,
		Since:     source.LastSeen.Add(-rule.TimeWindow),
		Until:     source.LastSeen.Add(rule.TimeWindow),
		Limit:     100,
	}

	if rule.TargetReason != "" {
		filter.Reason = rule.TargetReason
	}

	events, err := c.eventRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Filter out the source event itself
	var matches []storage.StoredEvent
	for _, e := range events {
		if e.ID != source.ID {
			matches = append(matches, e)
		}
	}

	return matches, nil
}

// matchesKindReason checks if an event matches kind/reason criteria.
func (c *Correlator) matchesKindReason(event storage.StoredEvent, kind, reason string) bool {
	if kind != "" && !strings.EqualFold(event.Kind, kind) {
		return false
	}
	if reason != "" && !strings.EqualFold(event.Reason, reason) {
		return false
	}
	return true
}

// calculateConfidence calculates correlation confidence.
func (c *Correlator) calculateConfidence(source, target storage.StoredEvent, rule CorrelationRule) float64 {
	baseConfidence := rule.Confidence

	// Adjust based on time proximity
	timeDiff := target.LastSeen.Sub(source.LastSeen)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	// Closer in time = higher confidence
	timeRatio := 1.0 - (float64(timeDiff) / float64(rule.TimeWindow))
	if timeRatio < 0.5 {
		baseConfidence *= 0.8
	} else if timeRatio > 0.8 {
		baseConfidence *= 1.1
		if baseConfidence > 1.0 {
			baseConfidence = 1.0
		}
	}

	// Same namespace boost
	if source.Namespace == target.Namespace {
		baseConfidence *= 1.1
		if baseConfidence > 1.0 {
			baseConfidence = 1.0
		}
	}

	// Warning events have higher correlation confidence
	if source.Type == "Warning" || target.Type == "Warning" {
		baseConfidence *= 1.05
		if baseConfidence > 1.0 {
			baseConfidence = 1.0
		}
	}

	return baseConfidence
}

// RunCorrelation runs correlation on recent events.
func (c *Correlator) RunCorrelation(ctx context.Context, since time.Time) (int, error) {
	events, err := c.eventRepo.List(ctx, storage.EventFilter{
		Since: since,
		Limit: 1000,
	})
	if err != nil {
		return 0, err
	}

	totalCorrelations := 0
	for _, event := range events {
		correlations, err := c.CorrelateEvent(ctx, event)
		if err != nil {
			continue
		}
		totalCorrelations += len(correlations)
	}

	return totalCorrelations, nil
}

// GetCorrelatedEvents gets events correlated to a specific event.
func (c *Correlator) GetCorrelatedEvents(ctx context.Context, eventID int64) ([]storage.CorrelationWithEvents, error) {
	// Get correlations where this event is the source
	sourceCorrs, err := c.corrRepo.FindBySource(ctx, eventID)
	if err != nil {
		return nil, err
	}

	// Get correlations where this event is the target
	targetCorrs, err := c.corrRepo.FindByTarget(ctx, eventID)
	if err != nil {
		return nil, err
	}

	// Combine and return
	all := make([]storage.CorrelationWithEvents, 0, len(sourceCorrs)+len(targetCorrs))
	all = append(all, sourceCorrs...)
	all = append(all, targetCorrs...)

	return all, nil
}

// AnalyzeIncident analyzes an incident by finding related events.
func (c *Correlator) AnalyzeIncident(ctx context.Context, cluster, namespace, kind, name string, since time.Time) (*IncidentAnalysis, error) {
	// Find events for the resource
	events, err := c.eventRepo.List(ctx, storage.EventFilter{
		Cluster:   cluster,
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
		Since:     since,
		Limit:     100,
	})
	if err != nil {
		return nil, err
	}

	analysis := &IncidentAnalysis{
		Resource:  kind + "/" + namespace + "/" + name,
		Since:     since,
		Events:    events,
		RootCause: nil,
	}

	// Find correlations for each event
	for _, event := range events {
		correlations, err := c.GetCorrelatedEvents(ctx, event.ID)
		if err != nil {
			continue
		}
		analysis.Correlations = append(analysis.Correlations, correlations...)
	}

	// Try to find root cause (highest confidence source event)
	if len(analysis.Correlations) > 0 {
		var bestCorr *storage.CorrelationWithEvents
		for i := range analysis.Correlations {
			corr := &analysis.Correlations[i]
			if bestCorr == nil || corr.Confidence > bestCorr.Confidence {
				bestCorr = corr
			}
		}
		if bestCorr != nil && bestCorr.Confidence > 0.7 {
			analysis.RootCause = &bestCorr.SourceEvent
			analysis.RootCauseConfidence = bestCorr.Confidence
		}
	}

	return analysis, nil
}

// IncidentAnalysis contains the results of incident analysis.
type IncidentAnalysis struct {
	Resource            string
	Since               time.Time
	Events              []storage.StoredEvent
	Correlations        []storage.CorrelationWithEvents
	RootCause           *storage.StoredEvent
	RootCauseConfidence float64
}
