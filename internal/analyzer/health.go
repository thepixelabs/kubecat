package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thepixelabs/kubecat/internal/client"
)

// HealthAnalyzer analyzes pod health issues like CrashLoopBackOff, OOMKilled, etc.
type HealthAnalyzer struct{}

// NewHealthAnalyzer creates a new health analyzer.
func NewHealthAnalyzer() *HealthAnalyzer {
	return &HealthAnalyzer{}
}

// Name returns the analyzer name.
func (a *HealthAnalyzer) Name() string {
	return "health"
}

// Category returns the issue category.
func (a *HealthAnalyzer) Category() Category {
	return CategoryConfig
}

// Analyze analyzes a single resource for health issues.
func (a *HealthAnalyzer) Analyze(ctx context.Context, cl client.ClusterClient, resource client.Resource) ([]Issue, error) {
	kind := strings.ToLower(resource.Kind)
	if kind != "pod" && kind != "pods" {
		return nil, nil
	}

	return a.analyzePod(resource)
}

// Scan scans all pods for health issues.
func (a *HealthAnalyzer) Scan(ctx context.Context, cl client.ClusterClient, namespace string) ([]Issue, error) {
	pods, err := cl.List(ctx, "pods", client.ListOptions{
		Namespace: namespace,
		Limit:     10000,
	})
	if err != nil {
		return nil, err
	}

	var allIssues []Issue
	for _, resource := range pods.Items {
		issues, _ := a.analyzePod(resource)
		allIssues = append(allIssues, issues...)
	}

	return allIssues, nil
}

func (a *HealthAnalyzer) analyzePod(resource client.Resource) ([]Issue, error) {
	var pod podSpec
	if err := json.Unmarshal(resource.Raw, &pod); err != nil {
		return nil, err
	}

	var issues []Issue

	// Check container statuses
	for _, cs := range pod.Status.ContainerStatuses {
		// Check for CrashLoopBackOff
		if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
			issues = append(issues, Issue{
				ID:       "health.crashloop",
				Category: CategoryConfig,
				Severity: SeverityCritical,
				Title:    "Container in CrashLoopBackOff",
				Message:  fmt.Sprintf("Container '%s' is in CrashLoopBackOff with %d restarts", cs.Name, cs.RestartCount),
				Resource: resource,
				Details: map[string]interface{}{
					"container":     cs.Name,
					"restart_count": cs.RestartCount,
					"message":       cs.State.Waiting.Message,
				},
				Fixes: []Fix{
					{Description: "Check container logs for errors", Command: fmt.Sprintf("kubectl logs %s -c %s -n %s --previous", pod.Metadata.Name, cs.Name, pod.Metadata.Namespace)},
					{Description: "Describe pod for more details", Command: fmt.Sprintf("kubectl describe pod %s -n %s", pod.Metadata.Name, pod.Metadata.Namespace)},
				},
				DetectedAt: time.Now(),
			})
		}

		// Check for ImagePullBackOff
		if cs.State.Waiting != nil && (cs.State.Waiting.Reason == "ImagePullBackOff" || cs.State.Waiting.Reason == "ErrImagePull") {
			issues = append(issues, Issue{
				ID:       "health.imagepull",
				Category: CategoryConfig,
				Severity: SeverityCritical,
				Title:    "Image pull failed",
				Message:  fmt.Sprintf("Container '%s' cannot pull image: %s", cs.Name, cs.State.Waiting.Message),
				Resource: resource,
				Details: map[string]interface{}{
					"container": cs.Name,
					"image":     cs.Image,
					"reason":    cs.State.Waiting.Reason,
					"message":   cs.State.Waiting.Message,
				},
				Fixes: []Fix{
					{Description: "Verify image name and tag exist"},
					{Description: "Check image pull secrets if using private registry"},
				},
				DetectedAt: time.Now(),
			})
		}

		// Check for OOMKilled
		if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
			issues = append(issues, Issue{
				ID:       "health.oomkilled",
				Category: CategoryConfig,
				Severity: SeverityWarning,
				Title:    "Container was OOMKilled",
				Message:  fmt.Sprintf("Container '%s' was killed due to out of memory", cs.Name),
				Resource: resource,
				Details: map[string]interface{}{
					"container":     cs.Name,
					"restart_count": cs.RestartCount,
					"exit_code":     cs.LastTerminationState.Terminated.ExitCode,
				},
				Fixes: []Fix{
					{Description: "Increase memory limits for the container"},
					{Description: "Investigate memory usage patterns in the application"},
				},
				DetectedAt: time.Now(),
			})
		}

		// Check for high restart count
		if cs.RestartCount >= 5 && cs.State.Running != nil {
			issues = append(issues, Issue{
				ID:       "health.highrestarts",
				Category: CategoryConfig,
				Severity: SeverityWarning,
				Title:    "High restart count",
				Message:  fmt.Sprintf("Container '%s' has restarted %d times", cs.Name, cs.RestartCount),
				Resource: resource,
				Details: map[string]interface{}{
					"container":     cs.Name,
					"restart_count": cs.RestartCount,
				},
				Fixes: []Fix{
					{Description: "Check logs for recurring errors", Command: fmt.Sprintf("kubectl logs %s -c %s -n %s", pod.Metadata.Name, cs.Name, pod.Metadata.Namespace)},
				},
				DetectedAt: time.Now(),
			})
		}
	}

	// Check for pod in Failed phase
	if pod.Status.Phase == "Failed" {
		reason := pod.Status.Reason
		if reason == "" {
			reason = "Unknown"
		}
		issues = append(issues, Issue{
			ID:       "health.failed",
			Category: CategoryConfig,
			Severity: SeverityCritical,
			Title:    "Pod in Failed state",
			Message:  fmt.Sprintf("Pod failed: %s - %s", reason, pod.Status.Message),
			Resource: resource,
			Details: map[string]interface{}{
				"reason":  reason,
				"message": pod.Status.Message,
			},
			Fixes: []Fix{
				{Description: "Check pod events and logs for failure reason"},
			},
			DetectedAt: time.Now(),
		})
	}

	return issues, nil
}

// WorkloadAnalyzer analyzes deployment and workload issues.
type WorkloadAnalyzer struct{}

// NewWorkloadAnalyzer creates a new workload analyzer.
func NewWorkloadAnalyzer() *WorkloadAnalyzer {
	return &WorkloadAnalyzer{}
}

// Name returns the analyzer name.
func (a *WorkloadAnalyzer) Name() string {
	return "workload"
}

// Category returns the issue category.
func (a *WorkloadAnalyzer) Category() Category {
	return CategoryConfig
}

// Analyze analyzes a single resource for workload issues.
func (a *WorkloadAnalyzer) Analyze(ctx context.Context, cl client.ClusterClient, resource client.Resource) ([]Issue, error) {
	kind := strings.ToLower(resource.Kind)
	switch kind {
	case "deployment", "deployments":
		return a.analyzeDeployment(resource)
	case "statefulset", "statefulsets":
		return a.analyzeStatefulSet(resource)
	case "daemonset", "daemonsets":
		return a.analyzeDaemonSet(resource)
	}
	return nil, nil
}

// Scan scans all workloads for issues.
func (a *WorkloadAnalyzer) Scan(ctx context.Context, cl client.ClusterClient, namespace string) ([]Issue, error) {
	var allIssues []Issue

	// Scan deployments
	deployments, err := cl.List(ctx, "deployments", client.ListOptions{Namespace: namespace, Limit: 1000})
	if err == nil {
		for _, r := range deployments.Items {
			issues, _ := a.analyzeDeployment(r)
			allIssues = append(allIssues, issues...)
		}
	}

	// Scan statefulsets
	statefulsets, err := cl.List(ctx, "statefulsets", client.ListOptions{Namespace: namespace, Limit: 1000})
	if err == nil {
		for _, r := range statefulsets.Items {
			issues, _ := a.analyzeStatefulSet(r)
			allIssues = append(allIssues, issues...)
		}
	}

	// Scan daemonsets
	daemonsets, err := cl.List(ctx, "daemonsets", client.ListOptions{Namespace: namespace, Limit: 1000})
	if err == nil {
		for _, r := range daemonsets.Items {
			issues, _ := a.analyzeDaemonSet(r)
			allIssues = append(allIssues, issues...)
		}
	}

	return allIssues, nil
}

func (a *WorkloadAnalyzer) analyzeDeployment(resource client.Resource) ([]Issue, error) {
	var dep deploymentSpec
	if err := json.Unmarshal(resource.Raw, &dep); err != nil {
		return nil, err
	}

	var issues []Issue

	// Check for unavailable replicas
	if dep.Status.UnavailableReplicas > 0 {
		issues = append(issues, Issue{
			ID:       "workload.unavailable",
			Category: CategoryConfig,
			Severity: SeverityWarning,
			Title:    "Deployment has unavailable replicas",
			Message:  fmt.Sprintf("%d of %d replicas unavailable", dep.Status.UnavailableReplicas, dep.Spec.Replicas),
			Resource: resource,
			Details: map[string]interface{}{
				"desired":     dep.Spec.Replicas,
				"ready":       dep.Status.ReadyReplicas,
				"unavailable": dep.Status.UnavailableReplicas,
			},
			Fixes: []Fix{
				{Description: "Check pod status", Command: fmt.Sprintf("kubectl get pods -l app=%s -n %s", dep.Metadata.Name, dep.Metadata.Namespace)},
			},
			DetectedAt: time.Now(),
		})
	}

	// Check for stuck rollout
	for _, cond := range dep.Status.Conditions {
		if cond.Type == "Progressing" && cond.Status == "False" && cond.Reason == "ProgressDeadlineExceeded" {
			issues = append(issues, Issue{
				ID:       "workload.rollout.stuck",
				Category: CategoryConfig,
				Severity: SeverityCritical,
				Title:    "Deployment rollout stuck",
				Message:  fmt.Sprintf("Rollout progress deadline exceeded: %s", cond.Message),
				Resource: resource,
				Details: map[string]interface{}{
					"reason":  cond.Reason,
					"message": cond.Message,
				},
				Fixes: []Fix{
					{Description: "Check rollout status", Command: fmt.Sprintf("kubectl rollout status deployment/%s -n %s", dep.Metadata.Name, dep.Metadata.Namespace)},
					{Description: "Rollback if needed", Command: fmt.Sprintf("kubectl rollout undo deployment/%s -n %s", dep.Metadata.Name, dep.Metadata.Namespace)},
				},
				DetectedAt: time.Now(),
			})
		}
	}

	// Check for zero replicas
	if dep.Spec.Replicas == 0 {
		issues = append(issues, Issue{
			ID:       "workload.zeroreplicas",
			Category: CategoryConfig,
			Severity: SeverityInfo,
			Title:    "Deployment scaled to zero",
			Message:  "Deployment has 0 replicas configured",
			Resource: resource,
			Fixes: []Fix{
				{Description: "Scale up if needed", Command: fmt.Sprintf("kubectl scale deployment/%s --replicas=1 -n %s", dep.Metadata.Name, dep.Metadata.Namespace)},
			},
			DetectedAt: time.Now(),
		})
	}

	return issues, nil
}

func (a *WorkloadAnalyzer) analyzeStatefulSet(resource client.Resource) ([]Issue, error) {
	var sts statefulSetSpec
	if err := json.Unmarshal(resource.Raw, &sts); err != nil {
		return nil, err
	}

	var issues []Issue

	if sts.Status.ReadyReplicas < sts.Spec.Replicas {
		issues = append(issues, Issue{
			ID:       "workload.statefulset.notready",
			Category: CategoryConfig,
			Severity: SeverityWarning,
			Title:    "StatefulSet not fully ready",
			Message:  fmt.Sprintf("%d of %d replicas ready", sts.Status.ReadyReplicas, sts.Spec.Replicas),
			Resource: resource,
			Details: map[string]interface{}{
				"desired": sts.Spec.Replicas,
				"ready":   sts.Status.ReadyReplicas,
			},
			DetectedAt: time.Now(),
		})
	}

	return issues, nil
}

func (a *WorkloadAnalyzer) analyzeDaemonSet(resource client.Resource) ([]Issue, error) {
	var ds daemonSetSpec
	if err := json.Unmarshal(resource.Raw, &ds); err != nil {
		return nil, err
	}

	var issues []Issue

	if ds.Status.NumberUnavailable > 0 {
		issues = append(issues, Issue{
			ID:       "workload.daemonset.unavailable",
			Category: CategoryConfig,
			Severity: SeverityWarning,
			Title:    "DaemonSet has unavailable pods",
			Message:  fmt.Sprintf("%d pods unavailable", ds.Status.NumberUnavailable),
			Resource: resource,
			Details: map[string]interface{}{
				"desired":     ds.Status.DesiredNumberScheduled,
				"ready":       ds.Status.NumberReady,
				"unavailable": ds.Status.NumberUnavailable,
			},
			DetectedAt: time.Now(),
		})
	}

	return issues, nil
}

// StorageAnalyzer analyzes storage-related issues.
type StorageAnalyzer struct{}

// NewStorageAnalyzer creates a new storage analyzer.
func NewStorageAnalyzer() *StorageAnalyzer {
	return &StorageAnalyzer{}
}

// Name returns the analyzer name.
func (a *StorageAnalyzer) Name() string {
	return "storage"
}

// Category returns the issue category.
func (a *StorageAnalyzer) Category() Category {
	return CategoryStorage
}

// Analyze analyzes a single resource for storage issues.
func (a *StorageAnalyzer) Analyze(ctx context.Context, cl client.ClusterClient, resource client.Resource) ([]Issue, error) {
	kind := strings.ToLower(resource.Kind)
	if kind != "persistentvolumeclaim" && kind != "persistentvolumeclaims" && kind != "pvc" {
		return nil, nil
	}

	return a.analyzePVC(resource)
}

// Scan scans all PVCs for storage issues.
func (a *StorageAnalyzer) Scan(ctx context.Context, cl client.ClusterClient, namespace string) ([]Issue, error) {
	pvcs, err := cl.List(ctx, "persistentvolumeclaims", client.ListOptions{
		Namespace: namespace,
		Limit:     1000,
	})
	if err != nil {
		return nil, err
	}

	var allIssues []Issue
	for _, r := range pvcs.Items {
		issues, _ := a.analyzePVC(r)
		allIssues = append(allIssues, issues...)
	}

	return allIssues, nil
}

func (a *StorageAnalyzer) analyzePVC(resource client.Resource) ([]Issue, error) {
	var pvc pvcSpec
	if err := json.Unmarshal(resource.Raw, &pvc); err != nil {
		return nil, err
	}

	var issues []Issue

	if pvc.Status.Phase == "Pending" {
		issues = append(issues, Issue{
			ID:       "storage.pvc.pending",
			Category: CategoryStorage,
			Severity: SeverityCritical,
			Title:    "PVC stuck in Pending",
			Message:  fmt.Sprintf("PVC '%s' is stuck in Pending state", pvc.Metadata.Name),
			Resource: resource,
			Details: map[string]interface{}{
				"storage_class": pvc.Spec.StorageClassName,
				"access_modes":  pvc.Spec.AccessModes,
			},
			Fixes: []Fix{
				{Description: "Check if StorageClass exists and has available capacity"},
				{Description: "Describe PVC for events", Command: fmt.Sprintf("kubectl describe pvc %s -n %s", pvc.Metadata.Name, pvc.Metadata.Namespace)},
			},
			DetectedAt: time.Now(),
		})
	}

	if pvc.Status.Phase == "Lost" {
		issues = append(issues, Issue{
			ID:       "storage.pvc.lost",
			Category: CategoryStorage,
			Severity: SeverityCritical,
			Title:    "PVC in Lost state",
			Message:  fmt.Sprintf("PVC '%s' has lost its bound PV", pvc.Metadata.Name),
			Resource: resource,
			Fixes: []Fix{
				{Description: "Check if the underlying PV still exists"},
				{Description: "May need to restore from backup"},
			},
			DetectedAt: time.Now(),
		})
	}

	return issues, nil
}

// NodeAnalyzer analyzes node health issues.
type NodeAnalyzer struct{}

// NewNodeAnalyzer creates a new node analyzer.
func NewNodeAnalyzer() *NodeAnalyzer {
	return &NodeAnalyzer{}
}

// Name returns the analyzer name.
func (a *NodeAnalyzer) Name() string {
	return "node"
}

// Category returns the issue category.
func (a *NodeAnalyzer) Category() Category {
	return CategoryNode
}

// Analyze analyzes a single resource for node issues.
func (a *NodeAnalyzer) Analyze(ctx context.Context, cl client.ClusterClient, resource client.Resource) ([]Issue, error) {
	kind := strings.ToLower(resource.Kind)
	if kind != "node" && kind != "nodes" {
		return nil, nil
	}

	return a.analyzeNode(resource)
}

// Scan scans all nodes for issues.
func (a *NodeAnalyzer) Scan(ctx context.Context, cl client.ClusterClient, namespace string) ([]Issue, error) {
	nodes, err := cl.List(ctx, "nodes", client.ListOptions{Limit: 1000})
	if err != nil {
		return nil, err
	}

	var allIssues []Issue
	for _, r := range nodes.Items {
		issues, _ := a.analyzeNode(r)
		allIssues = append(allIssues, issues...)
	}

	return allIssues, nil
}

func (a *NodeAnalyzer) analyzeNode(resource client.Resource) ([]Issue, error) {
	var node nodeSpec
	if err := json.Unmarshal(resource.Raw, &node); err != nil {
		return nil, err
	}

	var issues []Issue

	for _, cond := range node.Status.Conditions {
		switch cond.Type {
		case "Ready":
			if cond.Status != "True" {
				issues = append(issues, Issue{
					ID:       "node.notready",
					Category: CategoryNode,
					Severity: SeverityCritical,
					Title:    "Node not ready",
					Message:  fmt.Sprintf("Node '%s' is not ready: %s", node.Metadata.Name, cond.Message),
					Resource: resource,
					Details: map[string]interface{}{
						"reason":  cond.Reason,
						"message": cond.Message,
					},
					DetectedAt: time.Now(),
				})
			}
		case "MemoryPressure":
			if cond.Status == "True" {
				issues = append(issues, Issue{
					ID:         "node.memorypressure",
					Category:   CategoryNode,
					Severity:   SeverityWarning,
					Title:      "Node under memory pressure",
					Message:    fmt.Sprintf("Node '%s' is experiencing memory pressure", node.Metadata.Name),
					Resource:   resource,
					DetectedAt: time.Now(),
				})
			}
		case "DiskPressure":
			if cond.Status == "True" {
				issues = append(issues, Issue{
					ID:         "node.diskpressure",
					Category:   CategoryNode,
					Severity:   SeverityWarning,
					Title:      "Node under disk pressure",
					Message:    fmt.Sprintf("Node '%s' is experiencing disk pressure", node.Metadata.Name),
					Resource:   resource,
					DetectedAt: time.Now(),
				})
			}
		case "PIDPressure":
			if cond.Status == "True" {
				issues = append(issues, Issue{
					ID:         "node.pidpressure",
					Category:   CategoryNode,
					Severity:   SeverityWarning,
					Title:      "Node under PID pressure",
					Message:    fmt.Sprintf("Node '%s' is experiencing PID pressure", node.Metadata.Name),
					Resource:   resource,
					DetectedAt: time.Now(),
				})
			}
		case "NetworkUnavailable":
			if cond.Status == "True" {
				issues = append(issues, Issue{
					ID:         "node.networkunavailable",
					Category:   CategoryNode,
					Severity:   SeverityCritical,
					Title:      "Node network unavailable",
					Message:    fmt.Sprintf("Node '%s' network is unavailable", node.Metadata.Name),
					Resource:   resource,
					DetectedAt: time.Now(),
				})
			}
		}
	}

	return issues, nil
}

// Helper types for JSON parsing

type podSpec struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Status struct {
		Phase             string            `json:"phase"`
		Reason            string            `json:"reason"`
		Message           string            `json:"message"`
		ContainerStatuses []containerStatus `json:"containerStatuses"`
	} `json:"status"`
}

type containerStatus struct {
	Name                 string         `json:"name"`
	Image                string         `json:"image"`
	RestartCount         int            `json:"restartCount"`
	State                containerState `json:"state"`
	LastTerminationState containerState `json:"lastState"`
}

type containerState struct {
	Waiting    *waitingState    `json:"waiting,omitempty"`
	Running    *runningState    `json:"running,omitempty"`
	Terminated *terminatedState `json:"terminated,omitempty"`
}

type waitingState struct {
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type runningState struct {
	StartedAt string `json:"startedAt"`
}

type terminatedState struct {
	Reason   string `json:"reason"`
	Message  string `json:"message"`
	ExitCode int    `json:"exitCode"`
}

type deploymentSpec struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Replicas int `json:"replicas"`
	} `json:"spec"`
	Status struct {
		Replicas            int `json:"replicas"`
		ReadyReplicas       int `json:"readyReplicas"`
		AvailableReplicas   int `json:"availableReplicas"`
		UnavailableReplicas int `json:"unavailableReplicas"`
		Conditions          []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"conditions"`
	} `json:"status"`
}

type statefulSetSpec struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Replicas int `json:"replicas"`
	} `json:"spec"`
	Status struct {
		Replicas      int `json:"replicas"`
		ReadyReplicas int `json:"readyReplicas"`
	} `json:"status"`
}

type daemonSetSpec struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Status struct {
		DesiredNumberScheduled int `json:"desiredNumberScheduled"`
		NumberReady            int `json:"numberReady"`
		NumberUnavailable      int `json:"numberUnavailable"`
	} `json:"status"`
}

type pvcSpec struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		StorageClassName string   `json:"storageClassName"`
		AccessModes      []string `json:"accessModes"`
	} `json:"spec"`
	Status struct {
		Phase string `json:"phase"`
	} `json:"status"`
}

type nodeSpec struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Status struct {
		Conditions []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"conditions"`
	} `json:"status"`
}

func init() {
	Register(NewHealthAnalyzer())
	Register(NewWorkloadAnalyzer())
	Register(NewStorageAnalyzer())
	Register(NewNodeAnalyzer())
}
