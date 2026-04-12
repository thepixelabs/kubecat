// SPDX-License-Identifier: Apache-2.0

package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/thepixelabs/kubecat/internal/client"
)

// NetworkPolicyRecommendation holds a generated network policy YAML plus
// metadata describing why it was generated.
type NetworkPolicyRecommendation struct {
	// Namespace the policy applies to.
	Namespace string
	// Name is the suggested policy name.
	Name string
	// Description explains what the policy does.
	Description string
	// YAML is the generated network policy manifest.
	YAML string
	// Suppressed is true when the recommendation was suppressed (e.g. system ns).
	Suppressed bool
}

// NetpolRecommender generates NetworkPolicy recommendations from live cluster
// traffic observations (pod labels, services, ingresses).
type NetpolRecommender struct {
	cl client.ClusterClient
}

// NewNetpolRecommender creates a new recommender backed by the given client.
func NewNetpolRecommender(cl client.ClusterClient) *NetpolRecommender {
	return &NetpolRecommender{cl: cl}
}

// RecommendDefaultDeny generates a default-deny-all NetworkPolicy for the
// given namespace.  Returns a suppressed recommendation (no YAML) for
// well-known system namespaces.
func (r *NetpolRecommender) RecommendDefaultDeny(ctx context.Context, namespace string) (*NetworkPolicyRecommendation, error) {
	rec := &NetworkPolicyRecommendation{
		Namespace:   namespace,
		Name:        "default-deny-all",
		Description: fmt.Sprintf("Default deny-all ingress and egress in namespace %s", namespace),
	}

	// Suppress for system namespaces
	if isSystemNamespace(namespace) {
		rec.Suppressed = true
		return rec, nil
	}

	policy := buildDefaultDenyPolicy(namespace)
	yamlBytes, err := marshalPolicyYAML(policy)
	if err != nil {
		return nil, fmt.Errorf("marshal policy: %w", err)
	}
	rec.YAML = string(yamlBytes)
	return rec, nil
}

// RecommendForPod generates allow-policies for the given pod based on the
// services that select it and the ports it exposes.
func (r *NetpolRecommender) RecommendForPod(ctx context.Context, namespace, podName string) ([]*NetworkPolicyRecommendation, error) {
	if isSystemNamespace(namespace) {
		return nil, nil
	}

	pod, err := r.cl.Get(ctx, "pods", namespace, podName)
	if err != nil {
		return nil, fmt.Errorf("get pod: %w", err)
	}

	var podSpec struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Ports []struct {
					ContainerPort int    `json:"containerPort"`
					Protocol      string `json:"protocol"`
				} `json:"ports"`
			} `json:"containers"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(pod.Raw, &podSpec); err != nil {
		return nil, fmt.Errorf("unmarshal pod: %w", err)
	}

	var ports []int
	for _, c := range podSpec.Spec.Containers {
		for _, p := range c.Ports {
			ports = append(ports, p.ContainerPort)
		}
	}
	// Deduplicate and sort for deterministic output
	ports = uniqueSortedInts(ports)

	var recs []*NetworkPolicyRecommendation

	if len(ports) > 0 {
		rec := &NetworkPolicyRecommendation{
			Namespace:   namespace,
			Name:        fmt.Sprintf("allow-ingress-%s", podName),
			Description: fmt.Sprintf("Allow ingress to pod %s on ports %v", podName, ports),
		}
		policy := buildPodPortAllowPolicy(namespace, podName, podSpec.Metadata.Labels, ports)
		yamlBytes, err := marshalPolicyYAML(policy)
		if err != nil {
			return nil, fmt.Errorf("marshal pod-port policy: %w", err)
		}
		rec.YAML = string(yamlBytes)
		recs = append(recs, rec)
	}

	return recs, nil
}

// RecommendForService generates a NetworkPolicy that allows ingress to the
// pods selected by the given service.
func (r *NetpolRecommender) RecommendForService(ctx context.Context, namespace, serviceName string) (*NetworkPolicyRecommendation, error) {
	if isSystemNamespace(namespace) {
		return &NetworkPolicyRecommendation{Namespace: namespace, Suppressed: true}, nil
	}

	svc, err := r.cl.Get(ctx, "services", namespace, serviceName)
	if err != nil {
		return nil, fmt.Errorf("get service: %w", err)
	}

	var svcSpec struct {
		Spec struct {
			Selector map[string]string `json:"selector"`
			Ports    []struct {
				Port int `json:"port"`
			} `json:"ports"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(svc.Raw, &svcSpec); err != nil {
		return nil, fmt.Errorf("unmarshal service: %w", err)
	}

	var ports []int
	for _, p := range svcSpec.Spec.Ports {
		ports = append(ports, p.Port)
	}
	ports = uniqueSortedInts(ports)

	rec := &NetworkPolicyRecommendation{
		Namespace:   namespace,
		Name:        fmt.Sprintf("allow-svc-%s", serviceName),
		Description: fmt.Sprintf("Allow ingress to pods selected by Service %s", serviceName),
	}

	policy := buildServiceAllowPolicy(namespace, serviceName, svcSpec.Spec.Selector, ports)
	yamlBytes, err := marshalPolicyYAML(policy)
	if err != nil {
		return nil, fmt.Errorf("marshal service policy: %w", err)
	}
	rec.YAML = string(yamlBytes)
	return rec, nil
}

// ---------------------------------------------------------------------------
// policy builders
// ---------------------------------------------------------------------------

func buildDefaultDenyPolicy(namespace string) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata": map[string]interface{}{
			"name":      "default-deny-all",
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{},
			"policyTypes": []string{"Ingress", "Egress"},
		},
	}
}

func buildPodPortAllowPolicy(namespace, podName string, labels map[string]string, ports []int) map[string]interface{} {
	var portSpecs []map[string]interface{}
	for _, p := range ports {
		portSpecs = append(portSpecs, map[string]interface{}{"port": p})
	}

	matchLabels := map[string]interface{}{}
	for k, v := range labels {
		matchLabels[k] = v
	}

	return map[string]interface{}{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata": map[string]interface{}{
			"name":      fmt.Sprintf("allow-ingress-%s", podName),
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": matchLabels,
			},
			"policyTypes": []string{"Ingress"},
			"ingress": []map[string]interface{}{
				{"ports": portSpecs},
			},
		},
	}
}

func buildServiceAllowPolicy(namespace, serviceName string, selector map[string]string, ports []int) map[string]interface{} {
	var portSpecs []map[string]interface{}
	for _, p := range ports {
		portSpecs = append(portSpecs, map[string]interface{}{"port": p})
	}

	matchLabels := map[string]interface{}{}
	for k, v := range selector {
		matchLabels[k] = v
	}

	return map[string]interface{}{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata": map[string]interface{}{
			"name":      fmt.Sprintf("allow-svc-%s", serviceName),
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": matchLabels,
			},
			"policyTypes": []string{"Ingress"},
			"ingress": []map[string]interface{}{
				{"ports": portSpecs},
			},
		},
	}
}

// isSystemNamespace returns true for well-known Kubernetes system namespaces
// where network policy recommendations are suppressed.
func isSystemNamespace(namespace string) bool {
	switch namespace {
	case "kube-system", "kube-public", "kube-node-lease":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func marshalPolicyYAML(policy map[string]interface{}) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	enc.SetIndent(2)
	if err := enc.Encode(policy); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func uniqueSortedInts(in []int) []int {
	seen := make(map[int]struct{})
	var out []int
	for _, v := range in {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	sort.Ints(out)
	return out
}

// sortRecommendations sorts recommendations by name for deterministic output.
func sortRecommendations(recs []*NetworkPolicyRecommendation) {
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Namespace != recs[j].Namespace {
			return recs[i].Namespace < recs[j].Namespace
		}
		return strings.Compare(recs[i].Name, recs[j].Name) < 0
	})
}
