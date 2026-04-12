// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"encoding/json"
	"strings"
)

// Extract parses raw Kubernetes resource JSON into a ResourceMetadata struct.
// kind should be the lowercased plural form (e.g. "pods", "deployments").
func Extract(raw []byte, kind string) (*ResourceMetadata, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}

	m := &ResourceMetadata{
		Kind:  getString(obj, "kind"),
		Extra: make(map[string]string),
	}

	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		m.Name = getString(meta, "name")
		m.Namespace = getString(meta, "namespace")
		m.Labels = getStringMap(meta, "labels")
		m.Annotations = getStringMap(meta, "annotations")

		if owners, ok := meta["ownerReferences"].([]interface{}); ok && len(owners) > 0 {
			if owner, ok := owners[0].(map[string]interface{}); ok {
				m.OwnerKind = getString(owner, "kind")
				m.OwnerName = getString(owner, "name")
			}
		}
	}

	spec, _ := obj["spec"].(map[string]interface{})
	status, _ := obj["status"].(map[string]interface{})

	switch normalizeKind(kind) {
	case "pod":
		extractPod(m, spec, status)
	case "deployment":
		extractDeployment(m, spec, status)
	case "statefulset":
		extractStatefulSet(m, spec, status)
	case "daemonset":
		extractDaemonSet(m, spec, status)
	case "service":
		extractService(m, spec, status)
	case "persistentvolumeclaim":
		extractPVC(m, spec, status)
	case "ingress":
		extractIngress(m, spec, status)
	}

	return m, nil
}

func normalizeKind(kind string) string {
	k := strings.ToLower(strings.TrimSuffix(kind, "s"))
	switch k {
	case "po", "pod":
		return "pod"
	case "deploy", "deployment":
		return "deployment"
	case "sts", "statefulset":
		return "statefulset"
	case "ds", "daemonset":
		return "daemonset"
	case "svc", "service":
		return "service"
	case "pvc", "persistentvolumeclaim":
		return "persistentvolumeclaim"
	case "ing", "ingress":
		return "ingress"
	}
	return k
}

func extractPod(m *ResourceMetadata, spec, status map[string]interface{}) {
	if status != nil {
		m.PodPhase = getString(status, "phase")
		m.PodIP = getString(status, "podIP")
		m.Status = m.PodPhase

		var ready, total int
		if css, ok := status["containerStatuses"].([]interface{}); ok {
			total = len(css)
			for _, cs := range css {
				if cmap, ok := cs.(map[string]interface{}); ok {
					if restarts, ok := cmap["restartCount"].(float64); ok {
						m.RestartCount += int(restarts)
					}
					if r, ok := cmap["ready"].(bool); ok && r {
						ready++
					}
				}
			}
		}
		if total > 0 {
			m.ReadyContainers = formatFraction(ready, total)
		}
	}
	if spec != nil {
		m.NodeName = getString(spec, "nodeName")

		// Security checks on containers.
		if containers, ok := spec["containers"].([]interface{}); ok {
			for _, c := range containers {
				if cm, ok := c.(map[string]interface{}); ok {
					checkContainerSecurity(m, cm)
				}
			}
		}
	}
}

func extractDeployment(m *ResourceMetadata, spec, status map[string]interface{}) {
	if spec != nil {
		if r, ok := spec["replicas"].(float64); ok {
			m.Replicas = int32(r)
		}
	}
	if status != nil {
		if r, ok := status["readyReplicas"].(float64); ok {
			m.ReadyReplicas = int32(r)
		}
		if r, ok := status["availableReplicas"].(float64); ok {
			m.AvailableReplicas = int32(r)
		}
		if r, ok := status["updatedReplicas"].(float64); ok {
			m.UpdatedReplicas = int32(r)
		}
		m.Status = formatFraction(int(m.ReadyReplicas), int(m.Replicas))
	}
}

func extractStatefulSet(m *ResourceMetadata, spec, status map[string]interface{}) {
	extractDeployment(m, spec, status) // same fields
}

func extractDaemonSet(m *ResourceMetadata, _ map[string]interface{}, status map[string]interface{}) {
	if status != nil {
		desired, _ := status["desiredNumberScheduled"].(float64)
		ready, _ := status["numberReady"].(float64)
		m.Replicas = int32(desired)
		m.ReadyReplicas = int32(ready)
		m.Status = formatFraction(int(ready), int(desired))
	}
}

func extractService(m *ResourceMetadata, spec, status map[string]interface{}) {
	if spec == nil {
		return
	}
	m.ServiceType = getString(spec, "type")
	m.ClusterIP = getString(spec, "clusterIP")

	if ips, ok := spec["externalIPs"].([]interface{}); ok {
		for _, ip := range ips {
			if s, ok := ip.(string); ok {
				m.ExternalIPs = append(m.ExternalIPs, s)
			}
		}
	}

	if ports, ok := spec["ports"].([]interface{}); ok {
		for _, p := range ports {
			if pm, ok := p.(map[string]interface{}); ok {
				proto, _ := pm["protocol"].(string)
				port, _ := pm["port"].(float64)
				m.Ports = append(m.Ports, formatPort(int(port), proto))
			}
		}
	}
	_ = status
}

func extractPVC(m *ResourceMetadata, spec, status map[string]interface{}) {
	if spec != nil {
		m.StorageClass = getString(spec, "storageClassName")
		if am, ok := spec["accessModes"].([]interface{}); ok {
			for _, a := range am {
				if s, ok := a.(string); ok {
					m.AccessModes = append(m.AccessModes, s)
				}
			}
		}
	}
	if status != nil {
		m.Status = getString(status, "phase")
		if cap, ok := status["capacity"].(map[string]interface{}); ok {
			if storage, ok := cap["storage"].(string); ok {
				m.Capacity = storage
			}
		}
	}
}

func extractIngress(m *ResourceMetadata, spec, _ map[string]interface{}) {
	if spec == nil {
		return
	}
	if ic, ok := spec["ingressClassName"].(string); ok {
		m.IngressClass = ic
	}
	if rules, ok := spec["rules"].([]interface{}); ok {
		for _, rule := range rules {
			if rm, ok := rule.(map[string]interface{}); ok {
				host := getString(rm, "host")
				if host != "" {
					m.Hosts = append(m.Hosts, host)
				}
				if http, ok := rm["http"].(map[string]interface{}); ok {
					if paths, ok := http["paths"].([]interface{}); ok {
						for _, p := range paths {
							if pm, ok := p.(map[string]interface{}); ok {
								if backend, ok := pm["backend"].(map[string]interface{}); ok {
									if svc, ok := backend["service"].(map[string]interface{}); ok {
										m.Backends = append(m.Backends, getString(svc, "name"))
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if tls, ok := spec["tls"].([]interface{}); ok {
		for _, t := range tls {
			if tm, ok := t.(map[string]interface{}); ok {
				if hosts, ok := tm["hosts"].([]interface{}); ok {
					for _, h := range hosts {
						if s, ok := h.(string); ok {
							m.TLSHosts = append(m.TLSHosts, s)
						}
					}
				}
			}
		}
	}
}

func checkContainerSecurity(m *ResourceMetadata, container map[string]interface{}) {
	if sc, ok := container["securityContext"].(map[string]interface{}); ok {
		if priv, ok := sc["privileged"].(bool); ok && priv {
			m.SecurityIssues = append(m.SecurityIssues, "privileged")
		}
		if user, ok := sc["runAsUser"].(float64); ok && user == 0 {
			m.SecurityIssues = append(m.SecurityIssues, "root-user")
		}
	}
	if _, hasRes := container["resources"]; !hasRes {
		m.SecurityIssues = append(m.SecurityIssues, "no-resources")
	}
}

// helpers

func getString(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}

func getStringMap(m map[string]interface{}, key string) map[string]string {
	raw, ok := m[key].(map[string]interface{})
	if !ok {
		return nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		out[k], _ = v.(string)
	}
	return out
}

func formatFraction(a, b int) string {
	if b == 0 {
		return "0/0"
	}
	return strings.Join([]string{itoa(a), itoa(b)}, "/")
}

func formatPort(port int, proto string) string {
	if proto == "" || proto == "TCP" {
		return itoa(port)
	}
	return itoa(port) + "/" + proto
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// Reverse.
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
