// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"fmt"
	"strings"
)

// ResourceSummary is a minimal resource description used as input to ComputeEdges.
type ResourceSummary struct {
	Kind      string
	Name      string
	Namespace string
	Labels    map[string]string
	// Selectors is a "key=val, ..." string (for services).
	Selectors string
	// Backends is a "svc1:port, svc2:port" string (for ingresses).
	Backends string
	// OwnerKind / OwnerName are set from ownerReferences.
	OwnerKind string
	OwnerName string
}

// nodeID builds a canonical node identifier.
func nodeID(kind, namespace, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

// ComputeEdges derives edges between services, pods, ingresses, and
// replicasets/deployments from the supplied resource lists.
func ComputeEdges(
	services, pods, ingresses, replicasets []ResourceSummary,
) []Edge {
	var edges []Edge
	id := 0
	next := func() string {
		id++
		return fmt.Sprintf("edge-%d", id)
	}

	// Build RS → Deployment map for indirect pod ownership.
	rsToDeploy := make(map[string]string) // "namespace/rsName" → deployName
	for _, rs := range replicasets {
		if rs.OwnerKind == "Deployment" && rs.OwnerName != "" {
			rsToDeploy[rs.Namespace+"/"+rs.Name] = rs.OwnerName
		}
	}

	// Service → Pod edges via label selector.
	for _, svc := range services {
		if svc.Selectors == "" {
			continue
		}
		sel := ParseSelectors(svc.Selectors)
		if len(sel) == 0 {
			continue
		}
		srcID := nodeID("Service", svc.Namespace, svc.Name)
		for _, pod := range pods {
			if pod.Namespace != svc.Namespace {
				continue
			}
			if MatchLabels(pod.Labels, sel) {
				edges = append(edges, Edge{
					ID:       next(),
					Source:   srcID,
					Target:   nodeID("Pod", pod.Namespace, pod.Name),
					EdgeType: "service-to-pod",
				})
			}
		}
	}

	// Ingress → Service edges via backend references.
	for _, ing := range ingresses {
		if ing.Backends == "" {
			continue
		}
		ingID := nodeID("Ingress", ing.Namespace, ing.Name)
		for _, backend := range parseBackends(ing.Backends) {
			for _, svc := range services {
				if svc.Name == backend && svc.Namespace == ing.Namespace {
					edges = append(edges, Edge{
						ID:       next(),
						Source:   ingID,
						Target:   nodeID("Service", svc.Namespace, svc.Name),
						EdgeType: "ingress-to-service",
					})
					break
				}
			}
		}
	}

	// Controller → Pod edges via ownerReferences.
	for _, pod := range pods {
		if pod.OwnerKind == "" || pod.OwnerName == "" {
			continue
		}
		var ownerID string
		switch pod.OwnerKind {
		case "ReplicaSet":
			key := pod.Namespace + "/" + pod.OwnerName
			if deployName, ok := rsToDeploy[key]; ok {
				ownerID = nodeID("Deployment", pod.Namespace, deployName)
			}
		case "StatefulSet":
			ownerID = nodeID("StatefulSet", pod.Namespace, pod.OwnerName)
		case "DaemonSet":
			ownerID = nodeID("DaemonSet", pod.Namespace, pod.OwnerName)
		}
		if ownerID != "" {
			edges = append(edges, Edge{
				ID:       next(),
				Source:   ownerID,
				Target:   nodeID("Pod", pod.Namespace, pod.Name),
				EdgeType: "controller-to-pod",
			})
		}
	}

	return edges
}

// parseBackends extracts service names from "svc1:port, svc2:port".
func parseBackends(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if idx := strings.IndexByte(part, ':'); idx > 0 {
			part = part[:idx]
		}
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
