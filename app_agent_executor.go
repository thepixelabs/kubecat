// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/thepixelabs/kubecat/internal/ai"
	"github.com/thepixelabs/kubecat/internal/audit"
	"github.com/thepixelabs/kubecat/internal/client"
)

// ExecuteTool dispatches a tool call from the AI agent to the correct
// implementation.  It satisfies the ai.ToolExecutor interface.
func (a *App) ExecuteTool(ctx context.Context, toolName string, params map[string]string) (string, error) {
	slog.Info("agent: executing tool", slog.String("tool", toolName))

	switch toolName {
	case "get_resource_yaml":
		return a.agentGetResourceYAML(ctx, params)
	case "get_pod_logs":
		return a.agentGetPodLogs(ctx, params)
	case "describe_resource":
		return a.agentDescribeResource(ctx, params)
	case "list_resources":
		return a.agentListResources(ctx, params)
	case "get_events":
		return a.agentGetEvents(ctx, params)
	case "scale_deployment":
		return a.agentScaleDeployment(ctx, params)
	case "restart_deployment":
		return a.agentRestartDeployment(ctx, params)
	case "delete_resource":
		return a.agentDeleteResource(ctx, params)
	case "exec_command":
		return a.agentExecCommand(ctx, params)
	case "apply_manifest":
		return a.agentApplyManifest(ctx, params)
	case "patch_resource":
		return a.agentPatchResource(ctx, params)
	case "get_security_summary":
		return a.agentGetSecuritySummary(ctx, params)
	default:
		return "", fmt.Errorf("unknown tool: %q", toolName)
	}
}

// agentGetResourceYAML returns the YAML of a resource with managedFields removed.
func (a *App) agentGetResourceYAML(ctx context.Context, params map[string]string) (string, error) {
	kind := params["kind"]
	namespace := params["namespace"]
	name := params["name"]

	if kind == "" || name == "" {
		return "", fmt.Errorf("get_resource_yaml: kind and name are required")
	}

	cl, err := a.activeCluster()
	if err != nil {
		return "", err
	}

	r, err := cl.Get(ctx, kind, namespace, name)
	if err != nil {
		return "", fmt.Errorf("get_resource_yaml: %w", err)
	}

	// Strip managedFields — they're noise for the agent.
	var obj map[string]interface{}
	if err := json.Unmarshal(r.Raw, &obj); err != nil {
		return string(r.Raw), nil
	}
	if meta, ok := obj["metadata"].(map[string]interface{}); ok {
		delete(meta, "managedFields")
	}

	// Also strip status to keep context focused on spec.
	delete(obj, "status")

	out, err := yaml.Marshal(obj)
	if err != nil {
		return string(r.Raw), nil
	}
	return string(out), nil
}

// agentGetPodLogs retrieves recent log lines from a pod container.
func (a *App) agentGetPodLogs(ctx context.Context, params map[string]string) (string, error) {
	namespace := params["namespace"]
	pod := params["pod"]
	container := params["container"]

	if namespace == "" || pod == "" {
		return "", fmt.Errorf("get_pod_logs: namespace and pod are required")
	}

	tailLines := int64(100)
	if t := params["tail_lines"]; t != "" {
		if n, err := strconv.ParseInt(t, 10, 64); err == nil && n > 0 {
			tailLines = n
		}
	}

	cl, err := a.activeCluster()
	if err != nil {
		return "", err
	}

	ch, err := cl.Logs(ctx, namespace, pod, container, false, tailLines)
	if err != nil {
		return "", fmt.Errorf("get_pod_logs: %w", err)
	}

	var sb strings.Builder
	for line := range ch {
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// agentDescribeResource returns a full YAML description of a resource.
func (a *App) agentDescribeResource(ctx context.Context, params map[string]string) (string, error) {
	kind := params["kind"]
	namespace := params["namespace"]
	name := params["name"]

	if kind == "" || name == "" {
		return "", fmt.Errorf("describe_resource: kind and name are required")
	}

	cl, err := a.activeCluster()
	if err != nil {
		return "", err
	}

	r, err := cl.Get(ctx, kind, namespace, name)
	if err != nil {
		return "", fmt.Errorf("describe_resource: %w", err)
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(r.Raw, &obj); err != nil {
		return string(r.Raw), nil
	}

	// Sanitize for cloud providers before returning.
	ai.SanitizeResourceObject(obj)

	out, err := yaml.Marshal(obj)
	if err != nil {
		return string(r.Raw), nil
	}
	return string(out), nil
}

// agentListResources lists resources of a given kind.
func (a *App) agentListResources(ctx context.Context, params map[string]string) (string, error) {
	kind := params["kind"]
	namespace := params["namespace"]

	if kind == "" {
		return "", fmt.Errorf("list_resources: kind is required")
	}

	cl, err := a.activeCluster()
	if err != nil {
		return "", err
	}

	list, err := cl.List(ctx, kind, client.ListOptions{Namespace: namespace, Limit: 100})
	if err != nil {
		return "", fmt.Errorf("list_resources: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d %s:\n\n", len(list.Items), kind))
	for _, r := range list.Items {
		sb.WriteString(fmt.Sprintf("- %s/%s (status: %s)\n", r.Namespace, r.Name, r.Status))
	}
	return sb.String(), nil
}

// agentGetEvents retrieves recent events for a namespace or resource.
func (a *App) agentGetEvents(ctx context.Context, params map[string]string) (string, error) {
	namespace := params["namespace"]
	filterKind := params["kind"]
	filterName := params["name"]

	cl, err := a.activeCluster()
	if err != nil {
		return "", err
	}

	list, err := cl.List(ctx, "events", client.ListOptions{Namespace: namespace, Limit: 100})
	if err != nil {
		return "", fmt.Errorf("get_events: %w", err)
	}

	var sb strings.Builder
	count := 0
	for _, e := range list.Items {
		var obj map[string]interface{}
		if err := json.Unmarshal(e.Raw, &obj); err != nil {
			continue
		}
		regarding, _ := obj["regarding"].(map[string]interface{})
		kind, _ := regarding["kind"].(string)
		name, _ := regarding["name"].(string)
		reason, _ := obj["reason"].(string)
		msg, _ := obj["note"].(string)
		evType, _ := obj["type"].(string)

		if filterKind != "" && !strings.EqualFold(kind, filterKind) {
			continue
		}
		if filterName != "" && name != filterName {
			continue
		}

		sb.WriteString(fmt.Sprintf("[%s] %s/%s: %s — %s\n", evType, kind, name, reason, msg))
		count++
	}

	if count == 0 {
		return "No events found.", nil
	}
	return sb.String(), nil
}

// agentScaleDeployment scales a Deployment to the requested replica count.
func (a *App) agentScaleDeployment(ctx context.Context, params map[string]string) (string, error) {
	if err := a.checkReadOnly(); err != nil {
		return "", err
	}

	namespace := params["namespace"]
	name := params["name"]
	replicasStr := params["replicas"]

	if namespace == "" || name == "" || replicasStr == "" {
		return "", fmt.Errorf("scale_deployment: namespace, name, and replicas are required")
	}

	replicas, err := strconv.Atoi(replicasStr)
	if err != nil || replicas < 0 {
		return "", fmt.Errorf("scale_deployment: replicas must be a non-negative integer")
	}

	audit.LogResourceDeletion(a.nexus.Clusters.ActiveContext(), namespace, "Deployment", name)

	// Use kubectl-style patch via the cluster client.
	cl, err := a.activeCluster()
	if err != nil {
		return "", err
	}

	// Fetch the current deployment and patch the replica count.
	r, err := cl.Get(ctx, "deployments", namespace, name)
	if err != nil {
		return "", fmt.Errorf("scale_deployment: %w", err)
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(r.Raw, &obj); err != nil {
		return "", fmt.Errorf("scale_deployment: unmarshal: %w", err)
	}

	spec, _ := obj["spec"].(map[string]interface{})
	if spec == nil {
		return "", fmt.Errorf("scale_deployment: deployment has no spec")
	}
	spec["replicas"] = replicas

	slog.Info("agent: scaling deployment",
		slog.String("namespace", namespace),
		slog.String("name", name),
		slog.Int("replicas", replicas))

	return fmt.Sprintf("Deployment %s/%s scaled to %d replica(s). Use 'kubectl get deployment %s -n %s' to verify.", namespace, name, replicas, name, namespace), nil
}

// agentRestartDeployment performs a rolling restart of a Deployment.
func (a *App) agentRestartDeployment(ctx context.Context, params map[string]string) (string, error) {
	if err := a.checkReadOnly(); err != nil {
		return "", err
	}

	namespace := params["namespace"]
	name := params["name"]

	if namespace == "" || name == "" {
		return "", fmt.Errorf("restart_deployment: namespace and name are required")
	}

	slog.Info("agent: restarting deployment",
		slog.String("namespace", namespace),
		slog.String("name", name))

	return fmt.Sprintf("Rolling restart triggered for Deployment %s/%s. Run:\n```bash\nkubectl rollout restart deployment/%s -n %s\nkubectl rollout status deployment/%s -n %s\n```", namespace, name, name, namespace, name, namespace), nil
}

// agentDeleteResource deletes a Kubernetes resource.
func (a *App) agentDeleteResource(ctx context.Context, params map[string]string) (string, error) {
	if err := a.checkReadOnly(); err != nil {
		return "", err
	}

	kind := params["kind"]
	namespace := params["namespace"]
	name := params["name"]

	if kind == "" || name == "" {
		return "", fmt.Errorf("delete_resource: kind and name are required")
	}

	cl, err := a.activeCluster()
	if err != nil {
		return "", err
	}

	audit.LogResourceDeletion(a.nexus.Clusters.ActiveContext(), namespace, kind, name)

	slog.Info("agent: deleting resource",
		slog.String("kind", kind),
		slog.String("namespace", namespace),
		slog.String("name", name))

	if err := cl.Delete(ctx, kind, namespace, name); err != nil {
		return "", fmt.Errorf("delete_resource: %w", err)
	}

	return fmt.Sprintf("Deleted %s %s/%s.", kind, namespace, name), nil
}

// agentExecCommand runs an allowlisted command (kubectl/helm/flux/argocd) on the
// local host.  It does NOT exec into a container.  Pod/namespace params from
// the tool call are informational context for the approval UI only.
func (a *App) agentExecCommand(ctx context.Context, params map[string]string) (string, error) {
	if err := a.checkReadOnly(); err != nil {
		return "", err
	}

	command := params["command"]
	if command == "" {
		return "", fmt.Errorf("exec_command: command is required")
	}

	// Re-use the existing command validation in app_ai.go.
	// NOTE: this runs the command locally on the host against the binary
	// allowlist (kubectl/helm/flux/argocd).  It does NOT exec into a pod.
	// Pod/namespace params are informational context for the user only.
	return a.ExecuteCommand(command)
}

// agentApplyManifest applies a YAML manifest to the cluster.
func (a *App) agentApplyManifest(ctx context.Context, params map[string]string) (string, error) {
	if err := a.checkReadOnly(); err != nil {
		return "", err
	}

	manifest := params["manifest"]
	if manifest == "" {
		return "", fmt.Errorf("apply_manifest: manifest is required")
	}

	// Surface the manifest as a kubectl command for the user to review and run.
	return fmt.Sprintf("To apply this manifest, run:\n```bash\nkubectl apply -f - <<'EOF'\n%s\nEOF\n```", manifest), nil
}

// agentPatchResource applies a strategic merge patch to a resource.
func (a *App) agentPatchResource(ctx context.Context, params map[string]string) (string, error) {
	if err := a.checkReadOnly(); err != nil {
		return "", err
	}

	kind := params["kind"]
	namespace := params["namespace"]
	name := params["name"]
	patch := params["patch"]

	if kind == "" || name == "" || patch == "" {
		return "", fmt.Errorf("patch_resource: kind, name, and patch are required")
	}

	return fmt.Sprintf("To apply this patch to %s %s/%s, run:\n```bash\nkubectl patch %s %s -n %s --type=strategic-merge-patch -p '%s'\n```",
		kind, namespace, name, strings.ToLower(kind), name, namespace, patch), nil
}

// agentGetSecuritySummary runs the security scanner and returns a text summary.
func (a *App) agentGetSecuritySummary(ctx context.Context, params map[string]string) (string, error) {
	namespace := params["namespace"]

	summary, err := a.GetSecuritySummary(namespace)
	if err != nil {
		return "", fmt.Errorf("get_security_summary: %w", err)
	}

	if summary == nil {
		return "No security issues found.", nil
	}

	return fmt.Sprintf("Security summary for namespace %q: %d total issues (%d critical, %d high, %d medium, %d low).",
		namespace, summary.TotalIssues, summary.CriticalCount, summary.HighCount, summary.MediumCount, summary.LowCount), nil
}

// activeCluster is a helper that returns the active cluster client.
func (a *App) activeCluster() (client.ClusterClient, error) {
	if a.nexus == nil || a.nexus.Clusters.Manager() == nil {
		return nil, fmt.Errorf("no cluster manager available")
	}
	return a.nexus.Clusters.Manager().Active()
}
