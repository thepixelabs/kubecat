package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/transport/spdy"
)

// cluster implements ClusterClient for a single Kubernetes cluster.
type cluster struct {
	mu sync.RWMutex

	// contextName is the kubeconfig context name.
	contextName string

	// config is the REST configuration.
	config *rest.Config

	// clientset is the typed Kubernetes client.
	clientset kubernetes.Interface

	// dynamic is the dynamic client for arbitrary resources.
	dynamic dynamic.Interface

	// discovery is the discovery client for API resources.
	discovery discovery.DiscoveryInterface

	// info caches cluster information.
	info *ClusterInfo

	// gvrMu protects gvrCache.
	gvrMu sync.RWMutex

	// gvrCache caches GVR lookups.
	gvrCache map[string]schema.GroupVersionResource

	// closed indicates if the client has been closed.
	closed bool
}

// NewCluster creates a new cluster client for the given context.
func NewCluster(contextName string, config *rest.Config) (ClusterClient, error) {
	// Set reasonable defaults
	config.Timeout = 30 * time.Second
	config.QPS = 100
	config.Burst = 200

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &cluster{
		contextName: contextName,
		config:      config,
		clientset:   clientset,
		dynamic:     dynamicClient,
		discovery:   clientset.Discovery(),
		gvrCache:    make(map[string]schema.GroupVersionResource),
	}, nil
}

// Info returns information about the cluster.
func (c *cluster) Info(ctx context.Context) (*ClusterInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil, ErrNotConnected
	}

	info := &ClusterInfo{
		Name:      c.contextName,
		Context:   c.contextName,
		Server:    c.config.Host,
		LastCheck: time.Now(),
	}

	// Get server version
	version, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		// Format the error for better user feedback
		return nil, FormatConnectionError(err)
	}
	info.Version = version.GitVersion
	info.Status = StatusConnected

	// Get node count
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1000})
	if err == nil {
		info.NodeCount = len(nodes.Items)
	}

	// Get namespace count
	namespaces, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1000})
	if err == nil {
		info.NamespaceCount = len(namespaces.Items)
	}

	// Get pod count (approximate - limited query)
	pods, err := c.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{Limit: 10000})
	if err == nil {
		info.PodCount = len(pods.Items)
	}

	c.info = info
	return info, nil
}

// List lists resources of a given kind.
func (c *cluster) List(ctx context.Context, kind string, opts ListOptions) (*ResourceList, error) {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return nil, ErrNotConnected
	}

	gvr, err := c.resolveGVR(kind)
	if err != nil {
		return nil, err
	}

	listOpts := metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
		FieldSelector: opts.FieldSelector,
		Limit:         opts.Limit,
		Continue:      opts.Continue,
	}

	var list *unstructured.UnstructuredList
	if opts.Namespace != "" {
		list, err = c.dynamic.Resource(gvr).Namespace(opts.Namespace).List(ctx, listOpts)
	} else {
		list, err = c.dynamic.Resource(gvr).List(ctx, listOpts)
	}

	// Try fallback API versions for HelmReleases if the primary version fails
	if err != nil && (kind == "helmreleases" || kind == "helmrelease" || kind == "hr") {
		fmt.Printf("[DEBUG] HelmReleases primary (v2) failed: %v, trying fallbacks...\n", err)
		fallbackVersions := []string{"v2beta2", "v2beta1"}
		for _, version := range fallbackVersions {
			fallbackGVR := schema.GroupVersionResource{
				Group:    "helm.toolkit.fluxcd.io",
				Version:  version,
				Resource: "helmreleases",
			}
			if opts.Namespace != "" {
				list, err = c.dynamic.Resource(fallbackGVR).Namespace(opts.Namespace).List(ctx, listOpts)
			} else {
				list, err = c.dynamic.Resource(fallbackGVR).List(ctx, listOpts)
			}
			if err == nil {
				fmt.Printf("[DEBUG] HelmReleases fallback %s succeeded, found %d items\n", version, len(list.Items))
				break
			}
			fmt.Printf("[DEBUG] HelmReleases fallback %s failed: %v\n", version, err)
		}
	}

	// Try fallback API versions for Kustomizations if the primary version fails
	if err != nil && (kind == "kustomizations" || kind == "kustomization" || kind == "ks") {
		fmt.Printf("[DEBUG] Kustomizations primary (v1) failed: %v, trying fallbacks...\n", err)
		fallbackVersions := []string{"v1beta2", "v1beta1"}
		for _, version := range fallbackVersions {
			fallbackGVR := schema.GroupVersionResource{
				Group:    "kustomize.toolkit.fluxcd.io",
				Version:  version,
				Resource: "kustomizations",
			}
			if opts.Namespace != "" {
				list, err = c.dynamic.Resource(fallbackGVR).Namespace(opts.Namespace).List(ctx, listOpts)
			} else {
				list, err = c.dynamic.Resource(fallbackGVR).List(ctx, listOpts)
			}
			if err == nil {
				fmt.Printf("[DEBUG] Kustomizations fallback %s succeeded, found %d items\n", version, len(list.Items))
				break
			}
			fmt.Printf("[DEBUG] Kustomizations fallback %s failed: %v\n", version, err)
		}
	}

	if err != nil {
		return nil, err
	}

	resources := make([]Resource, 0, len(list.Items))
	for _, item := range list.Items {
		resources = append(resources, unstructuredToResource(&item))
	}

	return &ResourceList{
		Items:    resources,
		Total:    len(resources),
		Continue: list.GetContinue(),
	}, nil
}

// Get retrieves a single resource.
func (c *cluster) Get(ctx context.Context, kind, namespace, name string) (*Resource, error) {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return nil, ErrNotConnected
	}

	gvr, err := c.resolveGVR(kind)
	if err != nil {
		return nil, err
	}

	var obj *unstructured.Unstructured
	if namespace != "" {
		obj, err = c.dynamic.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = c.dynamic.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}

	// Try fallback API versions for HelmReleases if the primary version fails
	if err != nil && (kind == "helmreleases" || kind == "helmrelease" || kind == "hr") {
		fallbackVersions := []string{"v2beta2", "v2beta1"}
		for _, version := range fallbackVersions {
			fallbackGVR := schema.GroupVersionResource{
				Group:    "helm.toolkit.fluxcd.io",
				Version:  version,
				Resource: "helmreleases",
			}
			if namespace != "" {
				obj, err = c.dynamic.Resource(fallbackGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			} else {
				obj, err = c.dynamic.Resource(fallbackGVR).Get(ctx, name, metav1.GetOptions{})
			}
			if err == nil {
				break
			}
		}
	}

	// Try fallback API versions for Kustomizations if the primary version fails
	if err != nil && (kind == "kustomizations" || kind == "kustomization" || kind == "ks") {
		fallbackVersions := []string{"v1beta2", "v1beta1"}
		for _, version := range fallbackVersions {
			fallbackGVR := schema.GroupVersionResource{
				Group:    "kustomize.toolkit.fluxcd.io",
				Version:  version,
				Resource: "kustomizations",
			}
			if namespace != "" {
				obj, err = c.dynamic.Resource(fallbackGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			} else {
				obj, err = c.dynamic.Resource(fallbackGVR).Get(ctx, name, metav1.GetOptions{})
			}
			if err == nil {
				break
			}
		}
	}

	if err != nil {
		return nil, err
	}

	resource := unstructuredToResource(obj)
	return &resource, nil
}

// Delete deletes a resource.
func (c *cluster) Delete(ctx context.Context, kind, namespace, name string) error {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return ErrNotConnected
	}

	gvr, err := c.resolveGVR(kind)
	if err != nil {
		return err
	}

	if namespace != "" {
		return c.dynamic.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	}
	return c.dynamic.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
}

// Watch watches for resource changes.
func (c *cluster) Watch(ctx context.Context, kind string, opts WatchOptions) (<-chan WatchEvent, error) {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return nil, ErrNotConnected
	}

	gvr, err := c.resolveGVR(kind)
	if err != nil {
		return nil, err
	}

	watchOpts := metav1.ListOptions{
		LabelSelector:   opts.LabelSelector,
		ResourceVersion: opts.ResourceVersion,
		Watch:           true,
	}

	var watcher watch.Interface
	if opts.Namespace != "" {
		watcher, err = c.dynamic.Resource(gvr).Namespace(opts.Namespace).Watch(ctx, watchOpts)
	} else {
		watcher, err = c.dynamic.Resource(gvr).Watch(ctx, watchOpts)
	}
	if err != nil {
		return nil, err
	}

	events := make(chan WatchEvent, 100)
	go func() {
		defer close(events)
		defer watcher.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return
				}
				if obj, ok := event.Object.(*unstructured.Unstructured); ok {
					events <- WatchEvent{
						Type:     string(event.Type),
						Resource: unstructuredToResource(obj),
					}
				}
			}
		}
	}()

	return events, nil
}

// Logs streams logs from a pod.
func (c *cluster) Logs(ctx context.Context, namespace, pod, container string, follow bool, tailLines int64) (<-chan string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrNotConnected
	}

	opts := &corev1.PodLogOptions{
		Container: container,
		Follow:    follow,
	}
	if tailLines > 0 {
		opts.TailLines = &tailLines
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}

	logs := make(chan string, 100)
	go func() {
		defer close(logs)
		defer stream.Close()

		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case logs <- scanner.Text():
			}
		}
	}()

	return logs, nil
}

// Exec executes a command in a container.
func (c *cluster) Exec(ctx context.Context, namespace, pod, container string, command []string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return ErrNotConnected
	}

	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, metav1.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.config, http.MethodPost, req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	// Execute without interactive input
	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
}

// ExecInteractive executes a command with interactive I/O.
func (c *cluster) ExecInteractive(ctx context.Context, namespace, pod, container string, command []string, stdin io.Reader, stdout, stderr io.Writer, tty bool) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return ErrNotConnected
	}

	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdin:     stdin != nil,
		Stdout:    stdout != nil,
		Stderr:    stderr != nil,
		TTY:       tty,
	}, metav1.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.config, http.MethodPost, req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Tty:    tty,
	})
}

// PortForward creates a port forward to a pod.
func (c *cluster) PortForward(ctx context.Context, namespace, pod string, localPort, remotePort int) (PortForwarder, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrNotConnected
	}

	// Build the URL for the pod's port forward endpoint
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(c.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create round tripper: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, req.URL())

	// Create the port forward
	stopChan := make(chan struct{})
	readyChan := make(chan struct{})
	errChan := make(chan error, 1)
	doneChan := make(chan struct{})

	ports := []string{fmt.Sprintf("%d:%d", localPort, remotePort)}

	pf, err := portforward.New(dialer, ports, stopChan, readyChan, io.Discard, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("failed to create port forwarder: %w", err)
	}

	forwarder := &portForwarder{
		localPort:  localPort,
		remotePort: remotePort,
		stopChan:   stopChan,
		doneChan:   doneChan,
	}

	// Start port forwarding in a goroutine
	go func() {
		defer close(doneChan)
		if err := pf.ForwardPorts(); err != nil {
			forwarder.mu.Lock()
			forwarder.err = err
			forwarder.mu.Unlock()
			errChan <- err
		}
	}()

	// Wait for ready or error
	select {
	case <-readyChan:
		// Port forward is ready
		// Get the actual local port in case 0 was specified
		forwardedPorts, err := pf.GetPorts()
		if err == nil && len(forwardedPorts) > 0 {
			forwarder.localPort = int(forwardedPorts[0].Local)
		}
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		close(stopChan)
		return nil, ctx.Err()
	}

	return forwarder, nil
}

// portForwarder implements PortForwarder.
type portForwarder struct {
	localPort  int
	remotePort int
	stopChan   chan struct{}
	doneChan   chan struct{}
	mu         sync.RWMutex
	err        error
}

// LocalPort returns the local port being forwarded.
func (p *portForwarder) LocalPort() int {
	return p.localPort
}

// Stop stops the port forwarding.
func (p *portForwarder) Stop() {
	select {
	case <-p.stopChan:
		// Already closed
	default:
		close(p.stopChan)
	}
}

// Done returns a channel that's closed when the port forward ends.
func (p *portForwarder) Done() <-chan struct{} {
	return p.doneChan
}

// Error returns any error that occurred.
func (p *portForwarder) Error() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.err
}

// Close closes the client connection.
func (c *cluster) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true
	return nil
}

// resolveGVR resolves a resource kind to its GroupVersionResource.
// Note: Caller must NOT hold c.mu lock when calling this function.
func (c *cluster) resolveGVR(kind string) (schema.GroupVersionResource, error) {
	// Normalize to lowercase for lookup
	kind = strings.ToLower(kind)

	// Check cache first with read lock
	c.gvrMu.RLock()
	if gvr, ok := c.gvrCache[kind]; ok {
		c.gvrMu.RUnlock()
		return gvr, nil
	}
	c.gvrMu.RUnlock()

	// Common resource mappings
	commonGVRs := map[string]schema.GroupVersionResource{
		"pod":                    {Group: "", Version: "v1", Resource: "pods"},
		"pods":                   {Group: "", Version: "v1", Resource: "pods"},
		"po":                     {Group: "", Version: "v1", Resource: "pods"},
		"service":                {Group: "", Version: "v1", Resource: "services"},
		"services":               {Group: "", Version: "v1", Resource: "services"},
		"svc":                    {Group: "", Version: "v1", Resource: "services"},
		"namespace":              {Group: "", Version: "v1", Resource: "namespaces"},
		"namespaces":             {Group: "", Version: "v1", Resource: "namespaces"},
		"ns":                     {Group: "", Version: "v1", Resource: "namespaces"},
		"node":                   {Group: "", Version: "v1", Resource: "nodes"},
		"nodes":                  {Group: "", Version: "v1", Resource: "nodes"},
		"no":                     {Group: "", Version: "v1", Resource: "nodes"},
		"configmap":              {Group: "", Version: "v1", Resource: "configmaps"},
		"configmaps":             {Group: "", Version: "v1", Resource: "configmaps"},
		"cm":                     {Group: "", Version: "v1", Resource: "configmaps"},
		"secret":                 {Group: "", Version: "v1", Resource: "secrets"},
		"secrets":                {Group: "", Version: "v1", Resource: "secrets"},
		"deployment":             {Group: "apps", Version: "v1", Resource: "deployments"},
		"deployments":            {Group: "apps", Version: "v1", Resource: "deployments"},
		"deploy":                 {Group: "apps", Version: "v1", Resource: "deployments"},
		"dp":                     {Group: "apps", Version: "v1", Resource: "deployments"},
		"statefulset":            {Group: "apps", Version: "v1", Resource: "statefulsets"},
		"statefulsets":           {Group: "apps", Version: "v1", Resource: "statefulsets"},
		"sts":                    {Group: "apps", Version: "v1", Resource: "statefulsets"},
		"daemonset":              {Group: "apps", Version: "v1", Resource: "daemonsets"},
		"daemonsets":             {Group: "apps", Version: "v1", Resource: "daemonsets"},
		"ds":                     {Group: "apps", Version: "v1", Resource: "daemonsets"},
		"replicaset":             {Group: "apps", Version: "v1", Resource: "replicasets"},
		"replicasets":            {Group: "apps", Version: "v1", Resource: "replicasets"},
		"rs":                     {Group: "apps", Version: "v1", Resource: "replicasets"},
		"job":                    {Group: "batch", Version: "v1", Resource: "jobs"},
		"jobs":                   {Group: "batch", Version: "v1", Resource: "jobs"},
		"cronjob":                {Group: "batch", Version: "v1", Resource: "cronjobs"},
		"cronjobs":               {Group: "batch", Version: "v1", Resource: "cronjobs"},
		"cj":                     {Group: "batch", Version: "v1", Resource: "cronjobs"},
		"ingress":                {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		"ingresses":              {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		"ing":                    {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		"networkpolicy":          {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
		"networkpolicies":        {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
		"netpol":                 {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
		"persistentvolume":       {Group: "", Version: "v1", Resource: "persistentvolumes"},
		"persistentvolumes":      {Group: "", Version: "v1", Resource: "persistentvolumes"},
		"pv":                     {Group: "", Version: "v1", Resource: "persistentvolumes"},
		"persistentvolumeclaim":  {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		"persistentvolumeclaims": {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		"pvc":                    {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		"serviceaccount":         {Group: "", Version: "v1", Resource: "serviceaccounts"},
		"serviceaccounts":        {Group: "", Version: "v1", Resource: "serviceaccounts"},
		"sa":                     {Group: "", Version: "v1", Resource: "serviceaccounts"},
		"role":                   {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		"roles":                  {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
		"clusterrole":            {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		"clusterroles":           {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
		"rolebinding":            {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		"rolebindings":           {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		"clusterrolebinding":     {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
		"clusterrolebindings":    {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
		"event":                  {Group: "", Version: "v1", Resource: "events"},
		"events":                 {Group: "", Version: "v1", Resource: "events"},
		"ev":                     {Group: "", Version: "v1", Resource: "events"},
		// ArgoCD CRDs
		"application":     {Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"},
		"applications":    {Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"},
		"app":             {Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"},
		"appproject":      {Group: "argoproj.io", Version: "v1alpha1", Resource: "appprojects"},
		"appprojects":     {Group: "argoproj.io", Version: "v1alpha1", Resource: "appprojects"},
		"applicationset":  {Group: "argoproj.io", Version: "v1alpha1", Resource: "applicationsets"},
		"applicationsets": {Group: "argoproj.io", Version: "v1alpha1", Resource: "applicationsets"},
		// Flux CRDs
		"helmrelease":      {Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
		"helmreleases":     {Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
		"hr":               {Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
		"kustomization":    {Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
		"kustomizations":   {Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
		"ks":               {Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
		"gitrepository":    {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"},
		"gitrepositories":  {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"},
		"helmrepository":   {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmrepositories"},
		"helmrepositories": {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmrepositories"},
		"helmchart":        {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmcharts"},
		"helmcharts":       {Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmcharts"},
	}

	if gvr, ok := commonGVRs[kind]; ok {
		c.gvrMu.Lock()
		c.gvrCache[kind] = gvr
		c.gvrMu.Unlock()
		return gvr, nil
	}

	// TODO: Use discovery API for unknown resources
	return schema.GroupVersionResource{}, fmt.Errorf("unknown resource kind: %s", kind)
}

// unstructuredToResource converts an unstructured object to a Resource.
func unstructuredToResource(obj *unstructured.Unstructured) Resource {
	raw, _ := json.Marshal(obj.Object)
	createdAt := obj.GetCreationTimestamp().Time

	// Try to extract status
	status := "Unknown"
	if s, found, _ := unstructured.NestedString(obj.Object, "status", "phase"); found {
		status = s
	} else if conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions"); found && len(conditions) > 0 {
		if cond, ok := conditions[len(conditions)-1].(map[string]interface{}); ok {
			if t, ok := cond["type"].(string); ok {
				status = t
			}
		}
	}

	return Resource{
		Kind:        obj.GetKind(),
		APIVersion:  obj.GetAPIVersion(),
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		Labels:      obj.GetLabels(),
		Annotations: obj.GetAnnotations(),
		CreatedAt:   createdAt,
		Status:      status,
		Raw:         raw,
		Object:      obj.Object,
	}
}
