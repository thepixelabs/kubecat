package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thepixelabs/kubecat/internal/client"
)

var allowedKubectlVerbs = map[string]bool{
	"get": true, "describe": true, "logs": true, "top": true, "explain": true,
}

// Handler routes MCP tool calls to the underlying k8s client.
type Handler struct {
	manager client.Manager
}

// NewHandler creates a Handler backed by the given cluster manager.
func NewHandler(mgr client.Manager) *Handler {
	return &Handler{manager: mgr}
}

// Call dispatches a named tool call and returns the result as a string.
func (h *Handler) Call(ctx context.Context, tool string, args map[string]interface{}) (string, error) {
	switch tool {
	case "list_clusters":
		return h.listClusters()
	case "get_resource":
		return h.getResource(ctx, args)
	case "list_resources":
		return h.listResources(ctx, args)
	case "get_events":
		return h.getEvents(ctx, args)
	case "exec_kubectl":
		return h.execKubectl(args)
	case "ai_query":
		return h.aiQuery(args)
	default:
		return "", fmt.Errorf("unknown tool: %s", tool)
	}
}

func (h *Handler) listClusters() (string, error) {
	infos := h.manager.List()
	b, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *Handler) getResource(ctx context.Context, args map[string]interface{}) (string, error) {
	cluster, _ := args["cluster"].(string)
	namespace, _ := args["namespace"].(string)
	kind, _ := args["kind"].(string)
	name, _ := args["name"].(string)

	cl, err := h.clusterClient(cluster)
	if err != nil {
		return "", err
	}
	res, err := cl.Get(ctx, kind, namespace, name)
	if err != nil {
		return "", err
	}
	return string(res.Raw), nil
}

func (h *Handler) listResources(ctx context.Context, args map[string]interface{}) (string, error) {
	cluster, _ := args["cluster"].(string)
	namespace, _ := args["namespace"].(string)
	kind, _ := args["kind"].(string)

	cl, err := h.clusterClient(cluster)
	if err != nil {
		return "", err
	}
	list, err := cl.List(ctx, kind, client.ListOptions{Namespace: namespace, Limit: 100})
	if err != nil {
		return "", err
	}

	type summary struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Status    string `json:"status,omitempty"`
	}
	summaries := make([]summary, 0, len(list.Items))
	for _, r := range list.Items {
		summaries = append(summaries, summary{
			Name:      r.Name,
			Namespace: r.Namespace,
			Status:    r.Status,
		})
	}
	b, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *Handler) getEvents(ctx context.Context, args map[string]interface{}) (string, error) {
	cluster, _ := args["cluster"].(string)
	namespace, _ := args["namespace"].(string)
	resourceName, _ := args["resource_name"].(string)

	cl, err := h.clusterClient(cluster)
	if err != nil {
		return "", err
	}

	opts := client.ListOptions{Namespace: namespace, Limit: 50}
	if resourceName != "" {
		opts.FieldSelector = "involvedObject.name=" + resourceName
	}
	list, err := cl.List(ctx, "events", opts)
	if err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(list.Items, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (h *Handler) execKubectl(args map[string]interface{}) (string, error) {
	rawArgs, _ := args["args"].([]interface{})
	if len(rawArgs) == 0 {
		return "", fmt.Errorf("args must be non-empty")
	}
	verb, _ := rawArgs[0].(string)
	verb = strings.ToLower(verb)
	if !allowedKubectlVerbs[verb] {
		return "", fmt.Errorf("verb %q is not allowed; only read-only verbs are permitted: get, describe, logs, top, explain", verb)
	}
	return fmt.Sprintf("exec_kubectl is informational only in MCP mode; use list_resources or get_resource for programmatic access. Requested: kubectl %s",
		strings.Join(toStrings(rawArgs), " ")), nil
}

func (h *Handler) aiQuery(args map[string]interface{}) (string, error) {
	question, _ := args["question"].(string)
	if question == "" {
		return "", fmt.Errorf("question must not be empty")
	}
	return fmt.Sprintf("AI query received: %q — connect this to the kubecat AI pipeline via AIQueryWithContext Wails binding.", question), nil
}

func (h *Handler) clusterClient(contextName string) (client.ClusterClient, error) {
	if contextName == "" {
		return h.manager.Active()
	}
	cl, err := h.manager.Get(contextName)
	if err != nil {
		return h.manager.Active()
	}
	return cl, nil
}

func toStrings(in []interface{}) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
