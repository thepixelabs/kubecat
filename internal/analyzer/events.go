package analyzer

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// GetRelatedEvents retrieves events related to a specific resource.
func GetRelatedEvents(ctx context.Context, cl client.ClusterClient, resource client.Resource) ([]RelatedEvent, error) {
	// List events in the resource's namespace
	namespace := resource.Namespace
	if namespace == "" {
		namespace = "default"
	}

	eventList, err := cl.List(ctx, "events", client.ListOptions{
		Namespace: namespace,
		Limit:     1000,
	})
	if err != nil {
		return nil, err
	}

	var related []RelatedEvent
	for _, event := range eventList.Items {
		// Check if this event is related to our resource
		if isEventRelated(event, resource) {
			re, err := parseEvent(event)
			if err != nil {
				continue
			}
			related = append(related, re)
		}
	}

	// Sort by last timestamp descending (most recent first)
	sort.Slice(related, func(i, j int) bool {
		return related[i].LastTimestamp.After(related[j].LastTimestamp)
	})

	return related, nil
}

// isEventRelated checks if an event is related to a resource.
func isEventRelated(event client.Resource, resource client.Resource) bool {
	// Parse the event to check involvedObject
	var obj struct {
		InvolvedObject struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			UID       string `json:"uid"`
		} `json:"involvedObject"`
	}

	if err := json.Unmarshal(event.Raw, &obj); err != nil {
		return false
	}

	// Match by name and namespace
	if obj.InvolvedObject.Name == resource.Name &&
		obj.InvolvedObject.Namespace == resource.Namespace {
		return true
	}

	return false
}

// parseEvent parses a raw event into a RelatedEvent.
func parseEvent(event client.Resource) (RelatedEvent, error) {
	var obj struct {
		Type           string `json:"type"`
		Reason         string `json:"reason"`
		Message        string `json:"message"`
		Count          int    `json:"count"`
		FirstTimestamp string `json:"firstTimestamp"`
		LastTimestamp  string `json:"lastTimestamp"`
		EventTime      string `json:"eventTime"`
	}

	if err := json.Unmarshal(event.Raw, &obj); err != nil {
		return RelatedEvent{}, err
	}

	re := RelatedEvent{
		Type:    obj.Type,
		Reason:  obj.Reason,
		Message: obj.Message,
		Count:   obj.Count,
	}

	// Parse timestamps
	if obj.LastTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, obj.LastTimestamp); err == nil {
			re.LastTimestamp = t
		}
	} else if obj.EventTime != "" {
		if t, err := time.Parse(time.RFC3339, obj.EventTime); err == nil {
			re.LastTimestamp = t
		}
	}

	if obj.FirstTimestamp != "" {
		if t, err := time.Parse(time.RFC3339, obj.FirstTimestamp); err == nil {
			re.FirstTimestamp = t
		}
	}

	// Default count to 1
	if re.Count == 0 {
		re.Count = 1
	}

	return re, nil
}

// GetRecentEvents gets all recent events from a namespace.
func GetRecentEvents(ctx context.Context, cl client.ClusterClient, namespace string, since time.Duration) ([]RelatedEvent, error) {
	eventList, err := cl.List(ctx, "events", client.ListOptions{
		Namespace: namespace,
		Limit:     1000,
	})
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-since)
	var events []RelatedEvent

	for _, event := range eventList.Items {
		re, err := parseEvent(event)
		if err != nil {
			continue
		}

		// Filter by time
		if re.LastTimestamp.After(cutoff) || re.FirstTimestamp.After(cutoff) {
			events = append(events, re)
		}
	}

	// Sort by last timestamp descending
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTimestamp.After(events[j].LastTimestamp)
	})

	return events, nil
}

// GetWarningEvents gets only warning-type events.
func GetWarningEvents(ctx context.Context, cl client.ClusterClient, namespace string) ([]RelatedEvent, error) {
	events, err := GetRecentEvents(ctx, cl, namespace, 2*time.Hour)
	if err != nil {
		return nil, err
	}

	var warnings []RelatedEvent
	for _, e := range events {
		if e.Type == "Warning" {
			warnings = append(warnings, e)
		}
	}

	return warnings, nil
}
