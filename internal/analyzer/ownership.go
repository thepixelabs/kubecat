package analyzer

import (
	"context"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/thepixelabs/kubecat/internal/client"
)

// GetOwnerChain retrieves the ownership chain for a resource.
// It follows ownerReferences up to the root owner.
func GetOwnerChain(ctx context.Context, cl client.ClusterClient, resource client.Resource) (*OwnerChain, error) {
	chain := &OwnerChain{
		Resource: resource,
		Owners:   make([]OwnerRef, 0),
	}

	// Parse the raw JSON to get owner references
	owners, err := extractOwnerRefs(resource.Raw)
	if err != nil {
		return chain, nil // Return empty chain on error
	}

	// Follow the chain up
	for len(owners) > 0 {
		// Take the first controller owner (or first owner if no controller)
		var owner metav1.OwnerReference
		for _, o := range owners {
			if o.Controller != nil && *o.Controller {
				owner = o
				break
			}
		}
		if owner.Name == "" && len(owners) > 0 {
			owner = owners[0]
		}
		if owner.Name == "" {
			break
		}

		ownerRef := OwnerRef{
			Kind:       owner.Kind,
			Name:       owner.Name,
			Namespace:  resource.Namespace,
			APIVersion: owner.APIVersion,
			UID:        string(owner.UID),
		}

		// Try to get the owner resource
		kind := kindToPlural(owner.Kind)
		ownerResource, err := cl.Get(ctx, kind, resource.Namespace, owner.Name)
		if err == nil && ownerResource != nil {
			ownerRef.Resource = ownerResource
			// Get the next level of owners
			owners, _ = extractOwnerRefs(ownerResource.Raw)
		} else {
			owners = nil
		}

		chain.Owners = append(chain.Owners, ownerRef)
	}

	return chain, nil
}

// extractOwnerRefs extracts owner references from raw JSON.
func extractOwnerRefs(raw []byte) ([]metav1.OwnerReference, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var obj struct {
		Metadata struct {
			OwnerReferences []metav1.OwnerReference `json:"ownerReferences"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}

	return obj.Metadata.OwnerReferences, nil
}

// kindToPlural converts a Kind to its plural resource name.
func kindToPlural(kind string) string {
	// Common kind to plural mappings
	plurals := map[string]string{
		"Pod":                   "pods",
		"Deployment":            "deployments",
		"ReplicaSet":            "replicasets",
		"StatefulSet":           "statefulsets",
		"DaemonSet":             "daemonsets",
		"Job":                   "jobs",
		"CronJob":               "cronjobs",
		"Service":               "services",
		"ConfigMap":             "configmaps",
		"Secret":                "secrets",
		"PersistentVolumeClaim": "persistentvolumeclaims",
		"PersistentVolume":      "persistentvolumes",
		"Node":                  "nodes",
		"Namespace":             "namespaces",
		"ServiceAccount":        "serviceaccounts",
		"Ingress":               "ingresses",
		"NetworkPolicy":         "networkpolicies",
		"Role":                  "roles",
		"ClusterRole":           "clusterroles",
		"RoleBinding":           "rolebindings",
		"ClusterRoleBinding":    "clusterrolebindings",
	}

	if plural, ok := plurals[kind]; ok {
		return plural
	}

	// Default: lowercase and add 's'
	return kind + "s"
}
