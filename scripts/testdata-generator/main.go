// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	helmtime "helm.sh/helm/v3/pkg/time"
)

var (
	// Random source — math/rand for non-security sampling (list picks, jitter, etc.)
	rng = mathrand.New(mathrand.NewSource(time.Now().UnixNano()))

	// Common production labels
	commonLabels = map[string][]string{
		"app.kubernetes.io/version": {"1.0.0", "1.1.0", "1.2.0", "2.0.0", "2.1.0"},
		"app.kubernetes.io/env":     {"production", "staging", "development"},
		"team":                      {"platform", "backend", "frontend", "data", "security"},
		"cost-center":               {"engineering", "operations", "analytics"},
		"compliance":                {"pci", "hipaa", "sox", "gdpr"},
	}

	// Application stack names
	appStacks = []string{"payment-service", "user-service", "api-gateway", "auth-service", "notification-service", "analytics-engine"}

	// Container images
	containerImages = []string{
		"nginx:1.21",
		"redis:7.0-alpine",
		"postgres:14-alpine",
		"mongo:6.0",
		"elasticsearch:8.5.0",
		"rabbitmq:3.11-alpine",
		"gcr.io/mycompany/api-server:v2.1.0",
		"gcr.io/mycompany/frontend:v1.5.3",
		"gcr.io/mycompany/worker:v1.3.1",
	}
)

// randSecret generates a cryptographically random hex string of the given byte
// length (output length = 2*n characters). Use this instead of hardcoded
// credential strings so no static secret ever appears in source.
func randSecret(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failure is fatal — the caller would write an empty secret,
		// which is worse than crashing the test-data generator.
		panic(fmt.Sprintf("crypto/rand.Read: %v", err))
	}
	return []byte(hex.EncodeToString(b))
}

type Generator struct {
	client    *kubernetes.Clientset
	namespace string
	ctx       context.Context
}

func main() {
	fmt.Println("🚀 Kubernetes Test Data Generator")
	fmt.Println("=" + string(make([]byte, 50)))

	// Load kubeconfig
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("❌ Error loading kubeconfig: %v\n", err)
		os.Exit(1)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("❌ Error creating clientset: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Main menu
	for {
		fmt.Println("\n📋 Menu:")
		fmt.Println("1. Generate Complete Production-Like Stack")
		fmt.Println("2. Generate E-Commerce Application")
		fmt.Println("3. Generate Monitoring Stack")
		fmt.Println("4. Generate Data Processing Pipeline")
		fmt.Println("5. Generate Microservices Architecture")
		fmt.Println("6. Generate All (Full Kitchen Sink)")
		fmt.Println("7. Generate Helm Release")
		fmt.Println("8. Generate Kustomize App")
		fmt.Println("9. Clean Up Test Resources")
		fmt.Println("10. Exit")
		fmt.Print("\nSelect option: ")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			generateProductionStack(ctx, clientset)
		case 2:
			generateECommerceApp(ctx, clientset)
		case 3:
			generateMonitoringStack(ctx, clientset)
		case 4:
			generateDataPipeline(ctx, clientset)
		case 5:
			generateMicroservices(ctx, clientset)
		case 6:
			fmt.Println("\n🎯 Generating everything...")
			generateProductionStack(ctx, clientset)
			generateECommerceApp(ctx, clientset)
			generateMonitoringStack(ctx, clientset)
			generateDataPipeline(ctx, clientset)
			generateMicroservices(ctx, clientset)
			generateHelmStack(ctx, clientset)
			generateKustomizeStack(ctx, clientset)
			fmt.Println("\n✅ All stacks generated successfully!")
		case 7:
			generateHelmStack(ctx, clientset)
		case 8:
			generateKustomizeStack(ctx, clientset)
		case 9:
			cleanUpResources(ctx, clientset)
		case 10:
			fmt.Println("👋 Goodbye!")
			return
		default:
			fmt.Println("❌ Invalid option")
		}
	}
}

func generateProductionStack(ctx context.Context, client *kubernetes.Clientset) {
	fmt.Println("\n🏗️  Generating Production-Like Stack...")

	ns := "production"
	g := &Generator{client: client, namespace: ns, ctx: ctx}

	// Create namespace
	if err := g.createNamespace(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	// Generate resources
	g.createConfigMaps()
	g.createSecrets()
	g.createDeployments(3)
	g.createStatefulSets(2)
	g.createDaemonSets(2)
	g.createServices()
	g.createIngress()
	g.createJobs(2)

	fmt.Println("✅ Production stack generated!")
}

func generateECommerceApp(ctx context.Context, client *kubernetes.Clientset) {
	fmt.Println("\n🛒 Generating E-Commerce Application...")

	ns := "ecommerce"
	g := &Generator{client: client, namespace: ns, ctx: ctx}

	if err := g.createNamespace(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	// Frontend
	g.createDeploymentWithConfig("frontend-web", "nginx:1.21", 3, map[string]string{
		"app":       "frontend",
		"component": "web",
		"tier":      "frontend",
	})
	g.createServiceForApp("frontend-web", 80)

	// API Gateway
	g.createDeploymentWithConfig("api-gateway", "gcr.io/mycompany/api-gateway:v1.5.0", 2, map[string]string{
		"app":       "api-gateway",
		"component": "gateway",
		"tier":      "backend",
	})
	g.createServiceForApp("api-gateway", 8080)

	// Cart Service
	g.createDeploymentWithConfig("cart-service", "gcr.io/mycompany/cart:v2.0.0", 3, map[string]string{
		"app":       "cart-service",
		"component": "cart",
		"tier":      "backend",
	})
	g.createServiceForApp("cart-service", 8080)

	// Order Service
	g.createStatefulSetWithConfig("order-service", "gcr.io/mycompany/orders:v1.3.0", 3, map[string]string{
		"app":       "order-service",
		"component": "orders",
		"tier":      "backend",
	})
	g.createServiceForApp("order-service", 8080)

	// Payment Service
	g.createDeploymentWithConfig("payment-service", "gcr.io/mycompany/payment:v3.1.0", 2, map[string]string{
		"app":        "payment-service",
		"component":  "payment",
		"tier":       "backend",
		"compliance": "pci-dss",
	})
	g.createServiceForApp("payment-service", 8080)

	// Database
	g.createStatefulSetWithConfig("postgres-db", "postgres:14-alpine", 1, map[string]string{
		"app":       "database",
		"component": "postgres",
		"tier":      "data",
	})
	g.createServiceForApp("postgres-db", 5432)

	// Redis Cache
	g.createDeploymentWithConfig("redis-cache", "redis:7.0-alpine", 1, map[string]string{
		"app":       "cache",
		"component": "redis",
		"tier":      "cache",
	})
	g.createServiceForApp("redis-cache", 6379)

	// Ingress
	g.createIngressWithRules("ecommerce-ingress", map[string]string{
		"app": "ecommerce",
	}, []networkingv1.IngressRule{
		{
			Host: "shop.example.com",
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: pathTypePtr(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "frontend-web",
									Port: networkingv1.ServiceBackendPort{Number: 80},
								},
							},
						},
						{
							Path:     "/api",
							PathType: pathTypePtr(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: "api-gateway",
									Port: networkingv1.ServiceBackendPort{Number: 8080},
								},
							},
						},
					},
				},
			},
		},
	})

	g.createConfigMap("app-config", map[string]string{
		"ENVIRONMENT":     "production",
		"API_URL":         "http://api-gateway:8080",
		"CACHE_URL":       "redis://redis-cache:6379",
		"DB_HOST":         "postgres-db",
		"PAYMENT_GATEWAY": "stripe",
	})

	g.createSecret("app-secrets", map[string][]byte{
		"DB_PASSWORD":    randSecret(16),
		"STRIPE_API_KEY": randSecret(24),
		"JWT_SECRET":     randSecret(32),
		"REDIS_PASSWORD": randSecret(16),
	})

	fmt.Println("✅ E-Commerce application generated!")
}

func generateMonitoringStack(ctx context.Context, client *kubernetes.Clientset) {
	fmt.Println("\n📊 Generating Monitoring Stack...")

	ns := "monitoring"
	g := &Generator{client: client, namespace: ns, ctx: ctx}

	if err := g.createNamespace(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	// Prometheus
	g.createStatefulSetWithConfig("prometheus", "prom/prometheus:v2.40.0", 1, map[string]string{
		"app":       "prometheus",
		"component": "monitoring",
	})
	g.createServiceForApp("prometheus", 9090)

	// Grafana
	g.createDeploymentWithConfig("grafana", "grafana/grafana:9.3.0", 1, map[string]string{
		"app":       "grafana",
		"component": "visualization",
	})
	g.createServiceForApp("grafana", 3000)

	// Node Exporter (DaemonSet)
	g.createDaemonSetWithConfig("node-exporter", "prom/node-exporter:v1.5.0", map[string]string{
		"app":       "node-exporter",
		"component": "metrics",
	})
	g.createServiceForApp("node-exporter", 9100)

	// Loki
	g.createStatefulSetWithConfig("loki", "grafana/loki:2.7.0", 1, map[string]string{
		"app":       "loki",
		"component": "logging",
	})
	g.createServiceForApp("loki", 3100)

	// Promtail (DaemonSet)
	g.createDaemonSetWithConfig("promtail", "grafana/promtail:2.7.0", map[string]string{
		"app":       "promtail",
		"component": "log-collector",
	})

	// AlertManager
	g.createDeploymentWithConfig("alertmanager", "prom/alertmanager:v0.25.0", 1, map[string]string{
		"app":       "alertmanager",
		"component": "alerting",
	})
	g.createServiceForApp("alertmanager", 9093)

	g.createConfigMap("prometheus-config", map[string]string{
		"prometheus.yml": `
global:
  scrape_interval: 15s
scrape_configs:
  - job_name: 'kubernetes-pods'
    kubernetes_sd_configs:
    - role: pod
`,
	})

	fmt.Println("✅ Monitoring stack generated!")
}

func generateDataPipeline(ctx context.Context, client *kubernetes.Clientset) {
	fmt.Println("\n🔄 Generating Data Processing Pipeline...")

	ns := "data-pipeline"
	g := &Generator{client: client, namespace: ns, ctx: ctx}

	if err := g.createNamespace(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	// Kafka
	g.createStatefulSetWithConfig("kafka", "confluentinc/cp-kafka:7.3.0", 3, map[string]string{
		"app":       "kafka",
		"component": "message-broker",
	})
	g.createServiceForApp("kafka", 9092)

	// Zookeeper
	g.createStatefulSetWithConfig("zookeeper", "confluentinc/cp-zookeeper:7.3.0", 3, map[string]string{
		"app":       "zookeeper",
		"component": "coordination",
	})
	g.createServiceForApp("zookeeper", 2181)

	// Elasticsearch
	g.createStatefulSetWithConfig("elasticsearch", "elasticsearch:8.5.0", 3, map[string]string{
		"app":       "elasticsearch",
		"component": "search",
	})
	g.createServiceForApp("elasticsearch", 9200)

	// Data Processor Jobs
	g.createJobWithConfig("data-ingestion", "gcr.io/mycompany/data-ingest:v1.0.0", map[string]string{
		"app":       "data-ingestion",
		"component": "etl",
		"job-type":  "batch",
	})

	g.createJobWithConfig("data-transformation", "gcr.io/mycompany/data-transform:v2.1.0", map[string]string{
		"app":       "data-transformation",
		"component": "etl",
		"job-type":  "batch",
	})

	// Stream Processor
	g.createDeploymentWithConfig("stream-processor", "gcr.io/mycompany/stream-processor:v1.5.0", 3, map[string]string{
		"app":       "stream-processor",
		"component": "real-time",
	})

	fmt.Println("✅ Data pipeline generated!")
}

func generateMicroservices(ctx context.Context, client *kubernetes.Clientset) {
	fmt.Println("\n🎯 Generating Microservices Architecture...")

	ns := "microservices"
	g := &Generator{client: client, namespace: ns, ctx: ctx}

	if err := g.createNamespace(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	services := []struct {
		name     string
		replicas int32
		port     int
	}{
		{"user-service", 3, 8080},
		{"auth-service", 2, 8080},
		{"notification-service", 2, 8080},
		{"analytics-service", 2, 8080},
		{"search-service", 3, 8080},
		{"recommendation-service", 2, 8080},
	}

	for _, svc := range services {
		g.createDeploymentWithConfig(svc.name, randomImage(), svc.replicas, map[string]string{
			"app":     svc.name,
			"version": randomVersion(),
			"team":    randomTeam(),
		})
		g.createServiceForApp(svc.name, svc.port)
	}

	// Service Mesh sidecar injection simulation
	for _, svc := range services {
		g.addServiceMeshLabels(svc.name)
	}

	fmt.Println("✅ Microservices generated!")
}

func generateHelmStack(ctx context.Context, client *kubernetes.Clientset) {
	fmt.Println("\n⎈ Generating Helm Release...")

	ns := "helm-test"
	g := &Generator{client: client, namespace: ns, ctx: ctx}

	if err := g.createNamespace(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	// 1. Create the Helm Release object
	rlsName := "mysql-release"
	now := helmtime.Now()

	rls := &release.Release{
		Name:      rlsName,
		Namespace: ns,
		Version:   1,
		Info: &release.Info{
			FirstDeployed: now,
			LastDeployed:  now,
			Status:        release.StatusDeployed,
			Description:   "Install complete",
		},
		Chart: &chart.Chart{
			Metadata: &chart.Metadata{
				Name:       "mysql",
				Version:    "1.6.9",
				AppVersion: "5.7.30",
			},
		},
		Config: map[string]interface{}{
			"image": map[string]interface{}{
				"tag": "5.7.30",
			},
			"persistence": map[string]interface{}{
				"enabled": true,
				"size":    "8Gi",
			},
		},
		Manifest: `---
# Source: mysql/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: mysql-release
  namespace: helm-test
  labels:
    app.kubernetes.io/name: mysql
    app.kubernetes.io/instance: mysql-release
    app.kubernetes.io/managed-by: Helm
    helm.sh/chart: mysql-1.6.9
spec:
  ports:
  - name: mysql
    port: 3306
    targetPort: mysql
  selector:
    app.kubernetes.io/name: mysql
    app.kubernetes.io/instance: mysql-release
---
# Source: mysql/templates/statefulset.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: mysql-release
  namespace: helm-test
  labels:
    app.kubernetes.io/name: mysql
    app.kubernetes.io/instance: mysql-release
    app.kubernetes.io/managed-by: Helm
    helm.sh/chart: mysql-1.6.9
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: mysql
      app.kubernetes.io/instance: mysql-release
  serviceName: mysql-release
  replicas: 1
  template:
    metadata:
      labels:
        app.kubernetes.io/name: mysql
        app.kubernetes.io/instance: mysql-release
        app.kubernetes.io/managed-by: Helm
        helm.sh/chart: mysql-1.6.9
    spec:
      containers:
      - name: mysql
        image: "mysql:5.7.30"
        ports:
        - name: mysql
          containerPort: 3306
	`,
	}

	// 2. Create the Secret using Helm's storage driver
	// The driver expects a SecretInterface for the namespace where secrets are stored.
	secretsDriver := driver.NewSecrets(client.CoreV1().Secrets(ns))

	// Identify the key for the release (usually release name)
	// Helm storage creates secrets consistently named sh.helm.release.v1.<RELEASE_NAME>.v<VERSION>
	// The Create method handles the naming if we follow the pattern, but let's see how Create uses the key.
	// driver.Create(key, rls) - the key is used to check for uniqueness, but the underlying storage logic
	// (secrets.go) uses util.SecretName(key, release.Version) ? No, actually:
	// In the Secrets driver, Create creates a secret named <key> ?
	// Actually, Helm's action package handles the naming convention (sh.helm.release.v1...).
	// The Driver's Create method takes a key, but for the Secrets driver, the KEY is the SECRET NAME.
	// So we must manually construct the secret name compliant with Helm.

	secretName := fmt.Sprintf("sh.helm.release.v1.%s.v%d", rlsName, rls.Version)

	if err := secretsDriver.Create(secretName, rls); err != nil {
		fmt.Printf("⚠️  Error creating Helm secret (might already exist): %v\n", err)
	} else {
		fmt.Printf("  ✓ Created Helm Release Secret: %s\n", secretName)
	}

	// 3. Create the actual resources so they exist in the cluster
	// (Simulating what Helm would have done)

	// Service
	g.createServiceForApp("mysql-release", 3306)
	// We need to patch the labels to match Helm's expectations
	svc, err := client.CoreV1().Services(ns).Get(ctx, "mysql-release", metav1.GetOptions{})
	if err == nil {
		svc.Labels["app.kubernetes.io/managed-by"] = "Helm"
		svc.Labels["app.kubernetes.io/instance"] = rlsName
		svc.Labels["helm.sh/chart"] = "mysql-1.6.9"
		client.CoreV1().Services(ns).Update(ctx, svc, metav1.UpdateOptions{})
	}

	// StatefulSet
	g.createStatefulSetWithConfig("mysql-release", "mysql:5.7.30", 1, map[string]string{
		"app.kubernetes.io/name":       "mysql",
		"app.kubernetes.io/instance":   rlsName,
		"app.kubernetes.io/managed-by": "Helm",
		"helm.sh/chart":                "mysql-1.6.9",
	})

	fmt.Println("✅ Helm Release generated!")
}

func generateKustomizeStack(ctx context.Context, client *kubernetes.Clientset) {
	fmt.Println("\n🦎 Generating Kustomize App...")

	ns := "kustomize-app"
	g := &Generator{client: client, namespace: ns, ctx: ctx}

	if err := g.createNamespace(); err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	// Simulate Kustomize's content hash behavior
	configHash := "h92d1s7k92"
	configMapName := fmt.Sprintf("app-config-%s", configHash)

	// ConfigMap with hash
	g.createConfigMap(configMapName, map[string]string{
		"start-message": "Hello from Kustomize!",
	})

	// Add Kustomize specific annotations to the CM
	cm, _ := client.CoreV1().ConfigMaps(ns).Get(ctx, configMapName, metav1.GetOptions{})
	if cm != nil {
		cm.Annotations["kustomize.config.k8s.io/needs-hash"] = "true"
		client.CoreV1().ConfigMaps(ns).Update(ctx, cm, metav1.UpdateOptions{})
	}

	// Deployment referencing the hashed ConfigMap
	deployName := "podinfo"
	g.createDeploymentWithConfig(deployName, "ghcr.io/stefanprodan/podinfo:6.2.0", 2, map[string]string{
		"app":                          "podinfo",
		"app.kubernetes.io/managed-by": "kustomize", // Common label
	})

	// Patch deployment to simulate kustomize origin
	d, err := client.AppsV1().Deployments(ns).Get(ctx, deployName, metav1.GetOptions{})
	if err == nil {
		d.Annotations["kustomize.directory"] = "/base/podinfo"
		// Reference the hashed configmap in volumes (simplification)
		// in reality we would replace the volume source
		client.AppsV1().Deployments(ns).Update(ctx, d, metav1.UpdateOptions{})
	}

	g.createServiceForApp(deployName, 9898)

	fmt.Println("✅ Kustomize App generated!")
}

func cleanUpResources(ctx context.Context, client *kubernetes.Clientset) {
	fmt.Println("\n🧹 Cleaning up test resources...")

	namespaces := []string{"production", "ecommerce", "monitoring", "data-pipeline", "microservices", "helm-test", "kustomize-app"}

	for _, ns := range namespaces {
		fmt.Printf("Deleting namespace: %s...\n", ns)
		err := client.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("⚠️  Warning: %v\n", err)
		}
	}

	fmt.Println("✅ Cleanup complete!")
}

// Helper methods for Generator

func (g *Generator) createNamespace() error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: g.namespace,
			Labels: map[string]string{
				"environment": "test",
				"managed-by":  "testdata-generator",
				"created":     time.Now().Format("2006-01-02"),
			},
		},
	}

	_, err := g.client.CoreV1().Namespaces().Create(g.ctx, ns, metav1.CreateOptions{})
	if err != nil {
		// Ignore if already exists
		return nil
	}
	fmt.Printf("  ✓ Created namespace: %s\n", g.namespace)
	return nil
}

func (g *Generator) createConfigMaps() {
	configs := []struct {
		name string
		data map[string]string
	}{
		{
			"app-config",
			map[string]string{
				"LOG_LEVEL":   "info",
				"ENVIRONMENT": "production",
				"API_TIMEOUT": "30s",
				"MAX_RETRIES": "3",
			},
		},
		{
			"feature-flags",
			map[string]string{
				"NEW_UI_ENABLED":   "true",
				"BETA_FEATURES":    "false",
				"MAINTENANCE_MODE": "false",
			},
		},
	}

	for _, cfg := range configs {
		g.createConfigMap(cfg.name, cfg.data)
	}
}

func (g *Generator) createConfigMap(name string, data map[string]string) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.namespace,
			Labels:    g.generateLabels(name),
			Annotations: map[string]string{
				"config.kubernetes.io/checksum": fmt.Sprintf("%d", time.Now().Unix()),
			},
		},
		Data: data,
	}

	_, err := g.client.CoreV1().ConfigMaps(g.namespace).Create(g.ctx, cm, metav1.CreateOptions{})
	if err == nil {
		fmt.Printf("  ✓ Created ConfigMap: %s\n", name)
	}
}

func (g *Generator) createSecrets() {
	secrets := []struct {
		name string
		data map[string][]byte
	}{
		{
			"db-credentials",
			map[string][]byte{
				"username": []byte("admin"),
				"password": randSecret(16),
				"database": []byte("production_db"),
			},
		},
		{
			"api-keys",
			map[string][]byte{
				"stripe_key":   randSecret(24),
				"sendgrid_key": randSecret(24),
				"jwt_secret":   randSecret(32),
			},
		},
		{
			"tls-cert",
			map[string][]byte{
				"tls.crt": []byte("-----BEGIN CERTIFICATE-----\nMIIFakeCCAl..."),
				"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANB..."),
			},
		},
	}

	for _, secret := range secrets {
		g.createSecret(secret.name, secret.data)
	}
}

func (g *Generator) createSecret(name string, data map[string][]byte) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.namespace,
			Labels:    g.generateLabels(name),
			Annotations: map[string]string{
				"kubernetes.io/description": "Auto-generated test secret",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}

	_, err := g.client.CoreV1().Secrets(g.namespace).Create(g.ctx, secret, metav1.CreateOptions{})
	if err == nil {
		fmt.Printf("  ✓ Created Secret: %s\n", name)
	}
}

func (g *Generator) createDeployments(count int) {
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("app-deployment-%d", i+1)
		g.createDeploymentWithConfig(name, randomImage(), int32(rng.Intn(3)+1), g.generateLabels(name))
	}
}

func (g *Generator) createDeploymentWithConfig(name, image string, replicas int32, labels map[string]string) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"deployment.kubernetes.io/revision":                "1",
				"kubectl.kubernetes.io/last-applied-configuration": "{}",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: intStrPtr(intstr.FromInt(1)),
					MaxSurge:       intStrPtr(intstr.FromInt(1)),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: mergeMaps(labels, map[string]string{
						"app": name,
					}),
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   "8080",
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "ENVIRONMENT",
									Value: "production",
								},
								{
									Name: "POD_NAME",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								TimeoutSeconds:      5,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
								FailureThreshold:    3,
							},
						},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
						RunAsUser:    int64Ptr(1000),
						FSGroup:      int64Ptr(2000),
					},
				},
			},
		},
	}

	_, err := g.client.AppsV1().Deployments(g.namespace).Create(g.ctx, deployment, metav1.CreateOptions{})
	if err == nil {
		fmt.Printf("  ✓ Created Deployment: %s (replicas: %d)\n", name, replicas)
	}
}

func (g *Generator) createStatefulSets(count int) {
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("stateful-app-%d", i+1)
		g.createStatefulSetWithConfig(name, randomImage(), int32(rng.Intn(2)+1), g.generateLabels(name))
	}
}

func (g *Generator) createStatefulSetWithConfig(name, image string, replicas int32, labels map[string]string) {
	statefulset := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    int32Ptr(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: mergeMaps(labels, map[string]string{
						"app": name,
					}),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "data",
									MountPath: "/var/lib/data",
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							corev1.ReadWriteOnce,
						},
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("10Gi"),
							},
						},
					},
				},
			},
		},
	}

	_, err := g.client.AppsV1().StatefulSets(g.namespace).Create(g.ctx, statefulset, metav1.CreateOptions{})
	if err == nil {
		fmt.Printf("  ✓ Created StatefulSet: %s (replicas: %d)\n", name, replicas)
	}
}

func (g *Generator) createDaemonSets(count int) {
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("daemonset-app-%d", i+1)
		g.createDaemonSetWithConfig(name, randomImage(), g.generateLabels(name))
	}
}

func (g *Generator) createDaemonSetWithConfig(name, image string, labels map[string]string) {
	daemonset := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: mergeMaps(labels, map[string]string{
						"app": name,
					}),
				},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					HostPID:     true,
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: image,
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "host-root",
									MountPath: "/host",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "host-root",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/",
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "node-role.kubernetes.io/master",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
		},
	}

	_, err := g.client.AppsV1().DaemonSets(g.namespace).Create(g.ctx, daemonset, metav1.CreateOptions{})
	if err == nil {
		fmt.Printf("  ✓ Created DaemonSet: %s\n", name)
	}
}

func (g *Generator) createServices() {
	// Get all deployments and create services for them
	deployments, _ := g.client.AppsV1().Deployments(g.namespace).List(g.ctx, metav1.ListOptions{})
	for _, deploy := range deployments.Items {
		g.createServiceForApp(deploy.Name, 8080)
	}

	// Get all statefulsets and create services for them
	statefulsets, _ := g.client.AppsV1().StatefulSets(g.namespace).List(g.ctx, metav1.ListOptions{})
	for _, sts := range statefulsets.Items {
		g.createServiceForApp(sts.Name, 8080)
	}
}

func (g *Generator) createServiceForApp(appName string, port int) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: g.namespace,
			Labels: map[string]string{
				"app": appName,
			},
			Annotations: map[string]string{
				"service.beta.kubernetes.io/aws-load-balancer-type": "nlb",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": appName,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       int32(port),
					TargetPort: intstr.FromInt(port),
				},
			},
		},
	}

	_, err := g.client.CoreV1().Services(g.namespace).Create(g.ctx, service, metav1.CreateOptions{})
	if err == nil {
		fmt.Printf("  ✓ Created Service: %s:%d\n", appName, port)
	}
}

func (g *Generator) createIngress() {
	deployments, _ := g.client.AppsV1().Deployments(g.namespace).List(g.ctx, metav1.ListOptions{})
	if len(deployments.Items) == 0 {
		return
	}

	rules := []networkingv1.IngressRule{}
	for i, deploy := range deployments.Items {
		if i >= 3 {
			break
		}
		host := fmt.Sprintf("%s.example.com", deploy.Name)
		rules = append(rules, networkingv1.IngressRule{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: pathTypePtr(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: deploy.Name,
									Port: networkingv1.ServiceBackendPort{
										Number: 8080,
									},
								},
							},
						},
					},
				},
			},
		})
	}

	g.createIngressWithRules("main-ingress", map[string]string{
		"app": "main",
	}, rules)
}

func (g *Generator) createIngressWithRules(name string, labels map[string]string, rules []networkingv1.IngressRule) {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.namespace,
			Labels:    labels,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class":              "nginx",
				"cert-manager.io/cluster-issuer":           "letsencrypt-prod",
				"nginx.ingress.kubernetes.io/ssl-redirect": "true",
				"nginx.ingress.kubernetes.io/rate-limit":   "100",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: rules,
			TLS: []networkingv1.IngressTLS{
				{
					Hosts:      []string{"*.example.com"},
					SecretName: "tls-cert",
				},
			},
		},
	}

	_, err := g.client.NetworkingV1().Ingresses(g.namespace).Create(g.ctx, ingress, metav1.CreateOptions{})
	if err == nil {
		fmt.Printf("  ✓ Created Ingress: %s (%d rules)\n", name, len(rules))
	}
}

func (g *Generator) createJobs(count int) {
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("batch-job-%d", i+1)
		g.createJobWithConfig(name, randomImage(), g.generateLabels(name))
	}
}

func (g *Generator) createJobWithConfig(name, image string, labels map[string]string) {
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: g.namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			Completions:  int32Ptr(1),
			Parallelism:  int32Ptr(1),
			BackoffLimit: int32Ptr(3),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: mergeMaps(labels, map[string]string{
						"app": name,
					}),
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: image,
							Command: []string{
								"/bin/sh",
								"-c",
								"echo 'Processing data...'; sleep 30; echo 'Done!'",
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := g.client.BatchV1().Jobs(g.namespace).Create(g.ctx, job, metav1.CreateOptions{})
	if err == nil {
		fmt.Printf("  ✓ Created Job: %s\n", name)
	}
}

func (g *Generator) addServiceMeshLabels(appName string) {
	// This would add istio/linkerd labels in production
	// For now, just update the deployment with service mesh annotations
}

func (g *Generator) generateLabels(name string) map[string]string {
	labels := map[string]string{
		"app":                          name,
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/managed-by": "testdata-generator",
		"app.kubernetes.io/version":    randomVersion(),
		"team":                         randomTeam(),
		"environment":                  "test",
	}
	return labels
}

// Utility functions

func randomImage() string {
	return containerImages[rng.Intn(len(containerImages))]
}

func randomVersion() string {
	versions := commonLabels["app.kubernetes.io/version"]
	return versions[rng.Intn(len(versions))]
}

func randomTeam() string {
	teams := commonLabels["team"]
	return teams[rng.Intn(len(teams))]
}

func int32Ptr(i int32) *int32 {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func intStrPtr(is intstr.IntOrString) *intstr.IntOrString {
	return &is
}

func pathTypePtr(pt networkingv1.PathType) *networkingv1.PathType {
	return &pt
}

func mergeMaps(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}
