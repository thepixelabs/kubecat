package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/thepixelabs/kubecat/internal/audit"
)

// ResourceInfo is a JSON-friendly resource info.
type ResourceInfo struct {
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Status     string            `json:"status"`
	Age        string            `json:"age"`
	Labels     map[string]string `json:"labels"`
	APIVersion string            `json:"apiVersion"`

	// Extended metadata (flat structure for frontend)
	Replicas        string   `json:"replicas,omitempty"`        // e.g., "2/3" for deployments
	Restarts        int      `json:"restarts,omitempty"`        // pod restart count
	Node            string   `json:"node,omitempty"`            // node name for pods
	QoSClass        string   `json:"qosClass,omitempty"`        // pod QoS class
	ReadyContainers string   `json:"readyContainers,omitempty"` // e.g., "1/2"
	Images          []string `json:"images,omitempty"`          // container images
	CPURequest      string   `json:"cpuRequest,omitempty"`      // total CPU request
	CPULimit        string   `json:"cpuLimit,omitempty"`        // total CPU limit
	MemRequest      string   `json:"memRequest,omitempty"`      // total memory request
	MemLimit        string   `json:"memLimit,omitempty"`        // total memory limit
	ServiceType     string   `json:"serviceType,omitempty"`     // ClusterIP, NodePort, LoadBalancer
	ClusterIP       string   `json:"clusterIP,omitempty"`       // service cluster IP
	ExternalIP      string   `json:"externalIP,omitempty"`      // external IP/hostname
	Ports           string   `json:"ports,omitempty"`           // exposed ports
	StorageClass    string   `json:"storageClass,omitempty"`    // PVC storage class
	Capacity        string   `json:"capacity,omitempty"`        // PVC/PV capacity
	AccessModes     string   `json:"accessModes,omitempty"`     // PVC access modes
	IngressClass    string   `json:"ingressClass,omitempty"`    // ingress class name
	Hosts           string   `json:"hosts,omitempty"`           // ingress hosts
	Paths           string   `json:"paths,omitempty"`           // ingress paths
	TLSHosts        string   `json:"tlsHosts,omitempty"`        // ingress TLS hosts
	Backends        string   `json:"backends,omitempty"`        // ingress backend services
	Selectors       string   `json:"selectors,omitempty"`       // service pod selectors
	DataKeys        []string `json:"dataKeys,omitempty"`        // configmap/secret data keys
	DataCount       int      `json:"dataCount,omitempty"`       // number of data entries
	OwnerKind       string   `json:"ownerKind,omitempty"`       // owner reference kind
	OwnerName       string   `json:"ownerName,omitempty"`       // owner reference name
	HasLiveness     bool     `json:"hasLiveness,omitempty"`     // has liveness probe
	HasReadiness    bool     `json:"hasReadiness,omitempty"`    // has readiness probe
	SecurityIssues  []string `json:"securityIssues,omitempty"`  // security concerns
	Volumes         []string `json:"volumes,omitempty"`         // mounted volumes summary

	// Node-specific metadata
	CPUAllocatable   string   `json:"cpuAllocatable,omitempty"`   // node allocatable CPU
	MemAllocatable   string   `json:"memAllocatable,omitempty"`   // node allocatable memory
	CPUCapacity      string   `json:"cpuCapacity,omitempty"`      // node total CPU capacity
	MemCapacity      string   `json:"memCapacity,omitempty"`      // node total memory capacity
	PodCount         int      `json:"podCount,omitempty"`         // number of pods on node
	PodCapacity      int      `json:"podCapacity,omitempty"`      // max pods on node
	NodeConditions   []string `json:"nodeConditions,omitempty"`   // node conditions (Ready, MemoryPressure, etc)
	KubeletVersion   string   `json:"kubeletVersion,omitempty"`   // kubelet version
	ContainerRuntime string   `json:"containerRuntime,omitempty"` // container runtime
	OSImage          string   `json:"osImage,omitempty"`          // OS image
	Architecture     string   `json:"architecture,omitempty"`     // node architecture
	Taints           []string `json:"taints,omitempty"`           // node taints
	Unschedulable    bool     `json:"unschedulable,omitempty"`    // node is cordoned
	InternalIP       string   `json:"internalIP,omitempty"`       // node internal IP
	ExternalIPNode   string   `json:"externalIPNode,omitempty"`   // node external IP
	Roles            string   `json:"roles,omitempty"`            // node roles (master, worker, etc)
}

// ListResources lists resources of a given kind.
func (a *App) ListResources(kind, namespace string) ([]ResourceInfo, error) {
	resources, err := a.nexus.Resources.ListResources(a.ctx, kind, namespace)
	if err != nil {
		return nil, err
	}

	result := make([]ResourceInfo, len(resources))
	for i, r := range resources {
		info := a.nexus.Resources.GetResourceInfo(&r)
		result[i] = ResourceInfo{
			Kind:       info.Kind,
			Name:       info.Name,
			Namespace:  info.Namespace,
			Status:     info.Status,
			Age:        formatDuration(info.Age),
			Labels:     info.Labels,
			APIVersion: r.APIVersion,
		}
		// Extract extended metadata from raw JSON
		extractExtendedMetadata(&result[i], r.Raw, kind)
	}
	return result, nil
}

// GetResource gets a single resource as YAML/JSON.
func (a *App) GetResource(kind, namespace, name string) (string, error) {
	resource, err := a.nexus.Resources.GetResource(a.ctx, kind, namespace, name)
	if err != nil {
		return "", err
	}
	return string(resource.Raw), nil
}

// DeleteResource deletes a resource.
func (a *App) DeleteResource(kind, namespace, name string) error {
	if err := a.checkReadOnly(); err != nil {
		return err
	}
	slog.Info("deleting resource",
		slog.String("kind", kind),
		slog.String("namespace", namespace),
		slog.String("name", name),
	)
	audit.LogResourceDeletion(a.nexus.Clusters.ActiveContext(), namespace, kind, name)
	err := a.nexus.Resources.DeleteResource(a.ctx, kind, namespace, name)
	if err != nil {
		slog.Error("delete resource failed",
			slog.String("kind", kind),
			slog.String("namespace", namespace),
			slog.String("name", name),
			slog.Any("error", err),
		)
	}
	return err
}

// --- Cluster Graph Methods ---

// ClusterEdge represents a connection between two resources for visualization.
type ClusterEdge struct {
	ID       string `json:"id"`
	Source   string `json:"source"`   // nodeId: "kind/namespace/name"
	Target   string `json:"target"`   // nodeId: "kind/namespace/name"
	EdgeType string `json:"edgeType"` // "service-to-pod", "ingress-to-service", etc.
	Label    string `json:"label,omitempty"`
}

// GetClusterEdges computes edges between resources by analyzing selectors and references.
func (a *App) GetClusterEdges(namespace string) ([]ClusterEdge, error) {
	var edges []ClusterEdge
	edgeID := 0

	// Get services and pods to compute service-to-pod edges
	services, err := a.ListResources("services", namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	pods, err := a.ListResources("pods", namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Service -> Pod edges (via selector matching)
	for _, svc := range services {
		if svc.Selectors == "" {
			continue
		}
		// Parse selectors "key1=val1, key2=val2"
		selectorMap := parseSelectors(svc.Selectors)
		if len(selectorMap) == 0 {
			continue
		}

		svcNodeID := fmt.Sprintf("Service/%s/%s", svc.Namespace, svc.Name)

		for _, pod := range pods {
			if pod.Namespace != svc.Namespace {
				continue
			}
			if matchLabels(pod.Labels, selectorMap) {
				podNodeID := fmt.Sprintf("Pod/%s/%s", pod.Namespace, pod.Name)
				edges = append(edges, ClusterEdge{
					ID:       fmt.Sprintf("edge-%d", edgeID),
					Source:   svcNodeID,
					Target:   podNodeID,
					EdgeType: "service-to-pod",
				})
				edgeID++
			}
		}
	}

	// Get ingresses to compute ingress-to-service edges
	ingresses, err := a.ListResources("ingresses", namespace)
	if err != nil {
		// Ingresses might not exist, continue without error
		ingresses = []ResourceInfo{}
	}

	// Ingress -> Service edges (via backend references)
	for _, ing := range ingresses {
		if ing.Backends == "" {
			continue
		}
		ingNodeID := fmt.Sprintf("Ingress/%s/%s", ing.Namespace, ing.Name)

		// Parse backends "svc1:port, svc2:port"
		backends := parseBackends(ing.Backends)
		for _, backend := range backends {
			// Find matching service
			for _, svc := range services {
				if svc.Name == backend && svc.Namespace == ing.Namespace {
					svcNodeID := fmt.Sprintf("Service/%s/%s", svc.Namespace, svc.Name)
					edges = append(edges, ClusterEdge{
						ID:       fmt.Sprintf("edge-%d", edgeID),
						Source:   ingNodeID,
						Target:   svcNodeID,
						EdgeType: "ingress-to-service",
					})
					edgeID++
					break
				}
			}
		}
	}

	// Deployment/StatefulSet/DaemonSet -> Pod edges (via owner references)
	deployments, _ := a.ListResources("deployments", namespace)
	statefulsets, _ := a.ListResources("statefulsets", namespace)
	daemonsets, _ := a.ListResources("daemonsets", namespace)

	// Create a map of ReplicaSet -> Deployment for indirect ownership
	replicasets, _ := a.ListResources("replicasets", namespace)
	rsToDeployment := make(map[string]string) // rs name -> deployment name
	for _, rs := range replicasets {
		if rs.OwnerKind == "Deployment" && rs.OwnerName != "" {
			rsToDeployment[rs.Name] = rs.OwnerName
		}
	}

	for _, pod := range pods {
		podNodeID := fmt.Sprintf("Pod/%s/%s", pod.Namespace, pod.Name)

		// Direct ownership
		if pod.OwnerKind != "" && pod.OwnerName != "" {
			var ownerNodeID string
			switch pod.OwnerKind {
			case "ReplicaSet":
				// Check if RS is owned by Deployment
				if deployName, ok := rsToDeployment[pod.OwnerName]; ok {
					ownerNodeID = fmt.Sprintf("Deployment/%s/%s", pod.Namespace, deployName)
				}
			case "StatefulSet":
				ownerNodeID = fmt.Sprintf("StatefulSet/%s/%s", pod.Namespace, pod.OwnerName)
			case "DaemonSet":
				ownerNodeID = fmt.Sprintf("DaemonSet/%s/%s", pod.Namespace, pod.OwnerName)
			}

			if ownerNodeID != "" {
				edges = append(edges, ClusterEdge{
					ID:       fmt.Sprintf("edge-%d", edgeID),
					Source:   ownerNodeID,
					Target:   podNodeID,
					EdgeType: "controller-to-pod",
				})
				edgeID++
			}
		}
	}

	// Add unused controllers (deployments, statefulsets, daemonsets without edges yet)
	_ = deployments
	_ = statefulsets
	_ = daemonsets

	return edges, nil
}

// GetSecretData decodes a secret's data fields (base64) and returns them as plain strings.
func (a *App) GetSecretData(namespace, name string) (map[string]string, error) {
	audit.LogSecretAccess(a.nexus.Clusters.ActiveContext(), namespace, name)
	resource, err := a.nexus.Resources.GetResource(a.ctx, "secrets", namespace, name)
	if err != nil {
		return nil, err
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(resource.Raw, &obj); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	if data, ok := obj["data"].(map[string]interface{}); ok {
		for k, v := range data {
			if encoded, ok := v.(string); ok {
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				if err != nil {
					result[k] = fmt.Sprintf("[decode error: %v]", err)
				} else {
					result[k] = string(decoded)
				}
			}
		}
	}
	return result, nil
}

// parseSelectors parses "key1=val1, key2=val2" into a map
func parseSelectors(selectors string) map[string]string {
	result := make(map[string]string)
	if selectors == "" {
		return result
	}
	pairs := splitAndTrim(selectors, ",")
	for _, pair := range pairs {
		parts := splitAndTrim(pair, "=")
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// parseBackends parses "svc1:port, svc2:port" into service names
func parseBackends(backends string) []string {
	var result []string
	if backends == "" {
		return result
	}
	parts := splitAndTrim(backends, ",")
	for _, part := range parts {
		// Extract service name before ":"
		colonIdx := -1
		for i, c := range part {
			if c == ':' {
				colonIdx = i
				break
			}
		}
		if colonIdx > 0 {
			result = append(result, part[:colonIdx])
		} else {
			result = append(result, part)
		}
	}
	return result
}

// splitAndTrim splits a string and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			part := s[start:i]
			// Trim whitespace
			for len(part) > 0 && (part[0] == ' ' || part[0] == '\t') {
				part = part[1:]
			}
			for len(part) > 0 && (part[len(part)-1] == ' ' || part[len(part)-1] == '\t') {
				part = part[:len(part)-1]
			}
			if part != "" {
				result = append(result, part)
			}
			start = i + len(sep)
		}
	}
	// Last part
	part := s[start:]
	for len(part) > 0 && (part[0] == ' ' || part[0] == '\t') {
		part = part[1:]
	}
	for len(part) > 0 && (part[len(part)-1] == ' ' || part[len(part)-1] == '\t') {
		part = part[:len(part)-1]
	}
	if part != "" {
		result = append(result, part)
	}
	return result
}

// matchLabels checks if all selectors match the labels
func matchLabels(labels map[string]string, selectors map[string]string) bool {
	for k, v := range selectors {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// formatDuration formats a duration as a human-readable age.
func formatDuration(d interface{}) string {
	// Handle both time.Duration and string
	switch v := d.(type) {
	case interface{ Hours() float64 }:
		h := v.Hours()
		if h < 1 {
			return "<1h"
		}
		if h < 24 {
			return "<1d"
		}
		days := int(h / 24)
		return formatDays(days)
	default:
		return "unknown"
	}
}

func formatDays(days int) string {
	if days < 7 {
		return "<1w"
	}
	if days < 30 {
		return "<1mo"
	}
	if days < 365 {
		return "<1y"
	}
	return ">1y"
}

// extractExtendedMetadata extracts additional metadata from raw resource JSON.
func extractExtendedMetadata(info *ResourceInfo, raw []byte, kind string) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return
	}

	// Extract owner references
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		if ownerRefs, ok := metadata["ownerReferences"].([]interface{}); ok && len(ownerRefs) > 0 {
			if owner, ok := ownerRefs[0].(map[string]interface{}); ok {
				info.OwnerKind, _ = owner["kind"].(string)
				info.OwnerName, _ = owner["name"].(string)
			}
		}
	}

	spec, _ := obj["spec"].(map[string]interface{})
	status, _ := obj["status"].(map[string]interface{})

	switch kind {
	case "pods", "pod", "po":
		extractPodMetadata(info, spec, status)
	case "deployments", "deployment", "deploy", "dp":
		extractDeploymentMetadata(info, spec, status)
	case "statefulsets", "statefulset", "sts":
		extractStatefulSetMetadata(info, spec, status)
	case "daemonsets", "daemonset", "ds":
		extractDaemonSetMetadata(info, spec, status)
	case "services", "service", "svc":
		extractServiceMetadata(info, spec, status)
	case "persistentvolumeclaims", "persistentvolumeclaim", "pvc":
		extractPVCMetadata(info, spec, status)
	case "ingresses", "ingress", "ing":
		extractIngressMetadata(info, spec, status)
	case "replicasets", "replicaset", "rs":
		extractReplicaSetMetadata(info, spec, status)
	case "configmaps", "configmap", "cm":
		extractConfigMapMetadata(info, obj)
	case "secrets", "secret":
		extractSecretMetadata(info, obj)
	case "nodes", "node", "no":
		extractNodeMetadata(info, spec, status, obj)
	}
}

func extractPodMetadata(info *ResourceInfo, spec, status map[string]interface{}) {
	// Node name
	if nodeName, ok := spec["nodeName"].(string); ok {
		info.Node = nodeName
	}

	// QoS class
	if qos, ok := status["qosClass"].(string); ok {
		info.QoSClass = qos
	}

	// Container statuses for restarts and ready count
	var totalRestarts int
	var readyCount, totalCount int
	if containerStatuses, ok := status["containerStatuses"].([]interface{}); ok {
		totalCount = len(containerStatuses)
		for _, cs := range containerStatuses {
			if cstat, ok := cs.(map[string]interface{}); ok {
				if restarts, ok := cstat["restartCount"].(float64); ok {
					totalRestarts += int(restarts)
				}
				if ready, ok := cstat["ready"].(bool); ok && ready {
					readyCount++
				}
			}
		}
	}
	info.Restarts = totalRestarts
	if totalCount > 0 {
		info.ReadyContainers = fmt.Sprintf("%d/%d", readyCount, totalCount)
	}

	// Extract container info (images, resources, probes, security)
	if containers, ok := spec["containers"].([]interface{}); ok {
		var images []string
		var cpuReq, cpuLim, memReq, memLim int64
		var volumes []string
		var securityIssues []string

		for _, c := range containers {
			if container, ok := c.(map[string]interface{}); ok {
				// Image
				if image, ok := container["image"].(string); ok {
					images = append(images, image)
				}

				// Probes
				if _, ok := container["livenessProbe"]; ok {
					info.HasLiveness = true
				}
				if _, ok := container["readinessProbe"]; ok {
					info.HasReadiness = true
				}

				// Resources
				if resources, ok := container["resources"].(map[string]interface{}); ok {
					if requests, ok := resources["requests"].(map[string]interface{}); ok {
						cpuReq += parseResourceQuantity(requests["cpu"])
						memReq += parseResourceQuantity(requests["memory"])
					}
					if limits, ok := resources["limits"].(map[string]interface{}); ok {
						cpuLim += parseResourceQuantity(limits["cpu"])
						memLim += parseResourceQuantity(limits["memory"])
					}
				} else {
					securityIssues = append(securityIssues, "no-resources")
				}

				// Security context checks
				if secCtx, ok := container["securityContext"].(map[string]interface{}); ok {
					if priv, ok := secCtx["privileged"].(bool); ok && priv {
						securityIssues = append(securityIssues, "privileged")
					}

					if runAsUser, ok := secCtx["runAsUser"].(float64); ok && runAsUser == 0 {
						securityIssues = append(securityIssues, "root-user")
					}
				}
			}
		}

		info.Images = images
		if cpuReq > 0 {
			info.CPURequest = formatCPU(cpuReq)
		}
		if cpuLim > 0 {
			info.CPULimit = formatCPU(cpuLim)
		}
		if memReq > 0 {
			info.MemRequest = formatMemory(memReq)
		}
		if memLim > 0 {
			info.MemLimit = formatMemory(memLim)
		}
		info.Volumes = volumes
		info.SecurityIssues = securityIssues
	}

	// Check pod-level security context
	if podSecCtx, ok := spec["securityContext"].(map[string]interface{}); ok {
		if runAsUser, ok := podSecCtx["runAsUser"].(float64); ok && runAsUser == 0 {
			if !contains(info.SecurityIssues, "root-user") {
				info.SecurityIssues = append(info.SecurityIssues, "root-user")
			}
		}
	}

	// Volume info
	if vols, ok := spec["volumes"].([]interface{}); ok {
		var volSummary []string
		for _, v := range vols {
			if vol, ok := v.(map[string]interface{}); ok {
				name, _ := vol["name"].(string)
				if _, ok := vol["persistentVolumeClaim"]; ok {
					volSummary = append(volSummary, "pvc:"+name)
				} else if _, ok := vol["configMap"]; ok {
					volSummary = append(volSummary, "cm:"+name)
				} else if _, ok := vol["secret"]; ok {
					volSummary = append(volSummary, "secret:"+name)
				} else if _, ok := vol["emptyDir"]; ok {
					volSummary = append(volSummary, "empty:"+name)
				} else if _, ok := vol["hostPath"]; ok {
					volSummary = append(volSummary, "host:"+name)
					if !contains(info.SecurityIssues, "hostPath") {
						info.SecurityIssues = append(info.SecurityIssues, "hostPath")
					}
				}
			}
		}
		info.Volumes = volSummary
	}
}

func extractDeploymentMetadata(info *ResourceInfo, spec, status map[string]interface{}) {
	var desired, ready, available int
	if replicas, ok := spec["replicas"].(float64); ok {
		desired = int(replicas)
	}
	if r, ok := status["readyReplicas"].(float64); ok {
		ready = int(r)
	}
	if a, ok := status["availableReplicas"].(float64); ok {
		available = int(a)
	}
	info.Replicas = fmt.Sprintf("%d/%d", ready, desired)
	if available < desired {
		info.Status = fmt.Sprintf("%d/%d ready", available, desired)
	}

	// Extract images from pod template
	extractPodTemplateImages(info, spec)
}

func extractStatefulSetMetadata(info *ResourceInfo, spec, status map[string]interface{}) {
	var desired, ready int
	if replicas, ok := spec["replicas"].(float64); ok {
		desired = int(replicas)
	}
	if r, ok := status["readyReplicas"].(float64); ok {
		ready = int(r)
	}
	info.Replicas = fmt.Sprintf("%d/%d", ready, desired)

	extractPodTemplateImages(info, spec)
}

func extractDaemonSetMetadata(info *ResourceInfo, spec, status map[string]interface{}) {
	var desired, ready, available int
	if d, ok := status["desiredNumberScheduled"].(float64); ok {
		desired = int(d)
	}
	if r, ok := status["numberReady"].(float64); ok {
		ready = int(r)
	}
	if a, ok := status["numberAvailable"].(float64); ok {
		available = int(a)
	}
	info.Replicas = fmt.Sprintf("%d/%d", ready, desired)
	if available < desired {
		info.Status = fmt.Sprintf("%d/%d ready", available, desired)
	}

	extractPodTemplateImages(info, spec)
}

func extractServiceMetadata(info *ResourceInfo, spec, status map[string]interface{}) {
	if svcType, ok := spec["type"].(string); ok {
		info.ServiceType = svcType
	}
	if clusterIP, ok := spec["clusterIP"].(string); ok {
		info.ClusterIP = clusterIP
	}

	// Ports
	if ports, ok := spec["ports"].([]interface{}); ok {
		var portStrs []string
		for _, p := range ports {
			if port, ok := p.(map[string]interface{}); ok {
				portNum, _ := port["port"].(float64)
				protocol, _ := port["protocol"].(string)
				if protocol == "" {
					protocol = "TCP"
				}
				portStrs = append(portStrs, fmt.Sprintf("%d/%s", int(portNum), protocol))
			}
		}
		info.Ports = joinStrings(portStrs, ",")
	}

	// External IP for LoadBalancer
	if info.ServiceType == "LoadBalancer" {
		if lbStatus, ok := status["loadBalancer"].(map[string]interface{}); ok {
			if ingress, ok := lbStatus["ingress"].([]interface{}); ok && len(ingress) > 0 {
				if ing, ok := ingress[0].(map[string]interface{}); ok {
					if ip, ok := ing["ip"].(string); ok {
						info.ExternalIP = ip
					} else if hostname, ok := ing["hostname"].(string); ok {
						info.ExternalIP = hostname
					}
				}
			}
		}
	}

	// External IPs
	if extIPs, ok := spec["externalIPs"].([]interface{}); ok && len(extIPs) > 0 {
		var ips []string
		for _, ip := range extIPs {
			if ipStr, ok := ip.(string); ok {
				ips = append(ips, ipStr)
			}
		}
		if len(ips) > 0 {
			info.ExternalIP = joinStrings(ips, ",")
		}
	}

	// Pod selectors
	if selector, ok := spec["selector"].(map[string]interface{}); ok {
		var selectorStrs []string
		for k, v := range selector {
			if vStr, ok := v.(string); ok {
				selectorStrs = append(selectorStrs, fmt.Sprintf("%s=%s", k, vStr))
			}
		}
		info.Selectors = joinStrings(selectorStrs, ", ")
	}
}

func extractPVCMetadata(info *ResourceInfo, spec, status map[string]interface{}) {
	if sc, ok := spec["storageClassName"].(string); ok {
		info.StorageClass = sc
	}

	// Access modes
	if modes, ok := spec["accessModes"].([]interface{}); ok {
		var modeStrs []string
		for _, m := range modes {
			if mode, ok := m.(string); ok {
				// Abbreviate
				switch mode {
				case "ReadWriteOnce":
					modeStrs = append(modeStrs, "RWO")
				case "ReadOnlyMany":
					modeStrs = append(modeStrs, "ROX")
				case "ReadWriteMany":
					modeStrs = append(modeStrs, "RWX")
				default:
					modeStrs = append(modeStrs, mode)
				}
			}
		}
		info.AccessModes = joinStrings(modeStrs, ",")
	}

	// Capacity from status (actual) or spec (requested)
	if cap, ok := status["capacity"].(map[string]interface{}); ok {
		if storage, ok := cap["storage"].(string); ok {
			info.Capacity = storage
		}
	} else if resources, ok := spec["resources"].(map[string]interface{}); ok {
		if requests, ok := resources["requests"].(map[string]interface{}); ok {
			if storage, ok := requests["storage"].(string); ok {
				info.Capacity = storage
			}
		}
	}
}

func extractIngressMetadata(info *ResourceInfo, spec, status map[string]interface{}) {
	// Ingress class
	if class, ok := spec["ingressClassName"].(string); ok {
		info.IngressClass = class
	}

	// Extract hosts, paths, and backends from rules
	var hosts []string
	var paths []string
	var backends []string
	if rules, ok := spec["rules"].([]interface{}); ok {
		for _, r := range rules {
			if rule, ok := r.(map[string]interface{}); ok {
				if host, ok := rule["host"].(string); ok {
					hosts = append(hosts, host)
				}
				if http, ok := rule["http"].(map[string]interface{}); ok {
					if httpPaths, ok := http["paths"].([]interface{}); ok {
						for _, p := range httpPaths {
							if path, ok := p.(map[string]interface{}); ok {
								if pathStr, ok := path["path"].(string); ok {
									paths = append(paths, pathStr)
								}
								// Extract backend service
								if backend, ok := path["backend"].(map[string]interface{}); ok {
									if svc, ok := backend["service"].(map[string]interface{}); ok {
										svcName, _ := svc["name"].(string)
										if port, ok := svc["port"].(map[string]interface{}); ok {
											portNum, _ := port["number"].(float64)
											if svcName != "" {
												backends = append(backends, fmt.Sprintf("%s:%d", svcName, int(portNum)))
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	info.Hosts = joinStrings(hosts, ", ")
	info.Paths = joinStrings(paths, ", ")
	info.Backends = joinStrings(backends, ", ")

	// Extract TLS hosts
	var tlsHosts []string
	if tls, ok := spec["tls"].([]interface{}); ok {
		for _, t := range tls {
			if tlsEntry, ok := t.(map[string]interface{}); ok {
				if tlsHostList, ok := tlsEntry["hosts"].([]interface{}); ok {
					for _, h := range tlsHostList {
						if hostStr, ok := h.(string); ok {
							tlsHosts = append(tlsHosts, hostStr)
						}
					}
				}
			}
		}
	}
	info.TLSHosts = joinStrings(tlsHosts, ", ")
}

func extractReplicaSetMetadata(info *ResourceInfo, spec, status map[string]interface{}) {
	desired := 0
	if replicas, ok := spec["replicas"].(float64); ok {
		desired = int(replicas)
	}

	ready := 0
	if r, ok := status["readyReplicas"].(float64); ok {
		ready = int(r)
	}

	info.Replicas = fmt.Sprintf("%d/%d", ready, desired)

	// Extract images from pod template
	extractPodTemplateImages(info, spec)
}

func extractPodTemplateImages(info *ResourceInfo, spec map[string]interface{}) {
	if template, ok := spec["template"].(map[string]interface{}); ok {
		if podSpec, ok := template["spec"].(map[string]interface{}); ok {
			if containers, ok := podSpec["containers"].([]interface{}); ok {
				var images []string
				for _, c := range containers {
					if container, ok := c.(map[string]interface{}); ok {
						if image, ok := container["image"].(string); ok {
							images = append(images, image)
						}
					}
				}
				info.Images = images
			}
		}
	}
}

func extractConfigMapMetadata(info *ResourceInfo, obj map[string]interface{}) {
	// Extract data keys
	if data, ok := obj["data"].(map[string]interface{}); ok {
		var keys []string
		for k := range data {
			keys = append(keys, k)
		}
		info.DataKeys = keys
		info.DataCount = len(keys)
	}
	// Also check binaryData
	if binaryData, ok := obj["binaryData"].(map[string]interface{}); ok {
		for k := range binaryData {
			info.DataKeys = append(info.DataKeys, k+" (binary)")
		}
		info.DataCount += len(binaryData)
	}
}

func extractSecretMetadata(info *ResourceInfo, obj map[string]interface{}) {
	// Extract data keys (don't show values for security)
	if data, ok := obj["data"].(map[string]interface{}); ok {
		var keys []string
		for k := range data {
			keys = append(keys, k)
		}
		info.DataKeys = keys
		info.DataCount = len(keys)
	}
}

func extractNodeMetadata(info *ResourceInfo, spec, status, obj map[string]interface{}) {
	// Extract node roles from labels
	if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
		if labels, ok := metadata["labels"].(map[string]interface{}); ok {
			var roles []string
			for k := range labels {
				if len(k) > 24 && k[:24] == "node-role.kubernetes.io/" {
					roles = append(roles, k[24:])
				}
			}
			if len(roles) > 0 {
				info.Roles = joinStrings(roles, ",")
			}
		}
	}

	// Extract spec fields
	if unschedulable, ok := spec["unschedulable"].(bool); ok {
		info.Unschedulable = unschedulable
	}

	// Extract taints
	if taints, ok := spec["taints"].([]interface{}); ok {
		var taintStrs []string
		for _, t := range taints {
			if taint, ok := t.(map[string]interface{}); ok {
				key, _ := taint["key"].(string)
				value, _ := taint["value"].(string)
				effect, _ := taint["effect"].(string)
				if value != "" {
					taintStrs = append(taintStrs, fmt.Sprintf("%s=%s:%s", key, value, effect))
				} else {
					taintStrs = append(taintStrs, fmt.Sprintf("%s:%s", key, effect))
				}
			}
		}
		info.Taints = taintStrs
	}

	// Extract status fields
	if capacity, ok := status["capacity"].(map[string]interface{}); ok {
		if cpu, ok := capacity["cpu"].(string); ok {
			cpuMillis := parseResourceQuantity(cpu)
			if cpuMillis > 0 {
				info.CPUCapacity = formatCPU(cpuMillis * 1000) // capacity is in cores, convert to millicores
			}
		}
		if mem, ok := capacity["memory"].(string); ok {
			memBytes := parseResourceQuantity(mem)
			if memBytes > 0 {
				info.MemCapacity = formatMemory(memBytes)
			}
		}
		if pods, ok := capacity["pods"].(string); ok {
			var podCap int
			_, _ = fmt.Sscanf(pods, "%d", &podCap)
			info.PodCapacity = podCap
		}
	}

	if allocatable, ok := status["allocatable"].(map[string]interface{}); ok {
		if cpu, ok := allocatable["cpu"].(string); ok {
			cpuMillis := parseResourceQuantity(cpu)
			if cpuMillis > 0 {
				info.CPUAllocatable = formatCPU(cpuMillis * 1000) // allocatable is in cores, convert to millicores
			}
		}
		if mem, ok := allocatable["memory"].(string); ok {
			memBytes := parseResourceQuantity(mem)
			if memBytes > 0 {
				info.MemAllocatable = formatMemory(memBytes)
			}
		}
	}

	// Extract node conditions
	if conditions, ok := status["conditions"].([]interface{}); ok {
		var condStrs []string
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				condType, _ := cond["type"].(string)
				condStatus, _ := cond["status"].(string)
				// Include conditions that indicate problems
				if condType == "Ready" {
					if condStatus == "True" {
						condStrs = append(condStrs, "Ready")
					} else {
						condStrs = append(condStrs, "NotReady")
					}
				} else if condStatus == "True" {
					// For pressure conditions, True means problem
					condStrs = append(condStrs, condType)
				}
			}
		}
		info.NodeConditions = condStrs
	}

	// Extract node info
	if nodeInfo, ok := status["nodeInfo"].(map[string]interface{}); ok {
		if kubelet, ok := nodeInfo["kubeletVersion"].(string); ok {
			info.KubeletVersion = kubelet
		}
		if runtime, ok := nodeInfo["containerRuntimeVersion"].(string); ok {
			info.ContainerRuntime = runtime
		}
		if osImage, ok := nodeInfo["osImage"].(string); ok {
			info.OSImage = osImage
		}
		if arch, ok := nodeInfo["architecture"].(string); ok {
			info.Architecture = arch
		}
	}

	// Extract addresses
	if addresses, ok := status["addresses"].([]interface{}); ok {
		for _, a := range addresses {
			if addr, ok := a.(map[string]interface{}); ok {
				addrType, _ := addr["type"].(string)
				addrVal, _ := addr["address"].(string)
				if addrType == "InternalIP" {
					info.InternalIP = addrVal
				} else if addrType == "ExternalIP" {
					info.ExternalIPNode = addrVal
				}
			}
		}
	}
}

func parseResourceQuantity(val interface{}) int64 {
	if val == nil {
		return 0
	}
	str, ok := val.(string)
	if !ok {
		if num, ok := val.(float64); ok {
			return int64(num)
		}
		return 0
	}

	// Parse CPU (millicores)
	if len(str) > 0 && str[len(str)-1] == 'm' {
		var v int64
		_, _ = fmt.Sscanf(str, "%dm", &v)
		return v
	}

	// Parse memory
	multipliers := map[string]int64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024,
		"K":  1000,
		"M":  1000 * 1000,
		"G":  1000 * 1000 * 1000,
		"T":  1000 * 1000 * 1000 * 1000,
	}

	for suffix, mult := range multipliers {
		if len(str) > len(suffix) && str[len(str)-len(suffix):] == suffix {
			var v int64
			_, _ = fmt.Sscanf(str[:len(str)-len(suffix)], "%d", &v)
			return v * mult
		}
	}

	// Plain number (bytes for memory, cores for CPU)
	var v int64
	_, _ = fmt.Sscanf(str, "%d", &v)
	return v
}

func formatCPU(millicores int64) string {
	if millicores >= 1000 {
		return fmt.Sprintf("%.1f", float64(millicores)/1000)
	}
	return fmt.Sprintf("%dm", millicores)
}

func formatMemory(bytes int64) string {
	if bytes >= 1024*1024*1024 {
		return fmt.Sprintf("%.1fGi", float64(bytes)/(1024*1024*1024))
	}
	if bytes >= 1024*1024 {
		return fmt.Sprintf("%.0fMi", float64(bytes)/(1024*1024))
	}
	if bytes >= 1024 {
		return fmt.Sprintf("%.0fKi", float64(bytes)/1024)
	}
	return fmt.Sprintf("%d", bytes)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
