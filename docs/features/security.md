# Security Features

> **Status**: ✅ Available

Kubecat provides security-focused features for understanding and managing cluster security.

## Vision

Unified security visibility:

```
⎈ Security Center

Cluster Security Score: 72/100

┌─────────────────┬─────────────────┬─────────────────┐
│  RBAC Issues    │  Policy Viols   │  CVEs Found     │
│      12         │       5         │      23         │
│  ⚠ Medium       │  🔴 High        │  ⚠ Medium       │
└─────────────────┴─────────────────┴─────────────────┘
```

## Features

### RBAC Visualization

Understand who can access what:

```
RBAC Analysis: user/developer@example.com

Permissions in namespace 'default':
────────────────────────────────────────────────────────
✓ pods              get, list, watch, create, delete
✓ deployments       get, list, watch
✓ services          get, list, watch
✗ secrets           (no access)
✗ configmaps        (no access)

Effective Roles:
  ClusterRole: developer (via ClusterRoleBinding)
  Role: pod-manager (via RoleBinding in default)
```

### Permission Checker

Check if a user can perform an action:

```
> Can user/developer delete pods in production?

Checking: user/developer
Action:   delete
Resource: pods
Namespace: production

Result: ✗ Not allowed

Reason: No RoleBinding in namespace 'production' grants
        'delete' permission on 'pods'.

Similar permissions found:
  - Can delete pods in 'development'
  - Can delete pods in 'staging'
```

### Policy Dashboard

View OPA/Gatekeeper/Kyverno policy status:

```
Policy Violations

POLICY                  VIOLATIONS  SEVERITY  ACTION
────────────────────────────────────────────────────────
require-labels              12      Medium    Deny
restrict-host-network        3      High      Deny  
require-probes               8      Low       Warn
limit-replicas               2      Medium    Deny

Press Enter to view violation details
```

### CVE Tracking

Track vulnerabilities in running images:

```
Image Vulnerabilities

IMAGE                        CRITICAL  HIGH  MEDIUM  LOW
────────────────────────────────────────────────────────
nginx:1.19                        2       5      12     8
redis:6.2                         0       2       4     3
postgres:13                       1       3       8     5

Total: 3 critical, 10 high across 15 images

Press Enter to view CVE details
```

### Network Policy Viewer

Visualize network policies:

```
Network Policies for: pod/api-server

Ingress:
  ✓ Allow from pods with label app=frontend (port 8080)
  ✓ Allow from namespace monitoring (port 9090)
  ✗ Deny all other ingress

Egress:
  ✓ Allow to pods with label app=database (port 5432)
  ✓ Allow to external DNS (port 53)
  ✗ Deny all other egress
```

## Security Scoring

Overall cluster security score based on:

| Category | Weight | Checks |
|----------|--------|--------|
| RBAC | 25% | Least privilege, no wildcards |
| Policies | 25% | Coverage, enforcement |
| Images | 20% | CVE counts, age |
| Network | 15% | Policy coverage |
| Secrets | 15% | Encryption, rotation |

### Score Breakdown

```
Security Score: 72/100

Category Scores:
  RBAC:     85/100  ████████░░
  Policies: 60/100  ██████░░░░
  Images:   65/100  ██████░░░░
  Network:  80/100  ████████░░
  Secrets:  70/100  ███████░░░

Top Issues:
  1. 5 pods running as root
  2. 3 service accounts with cluster-admin
  3. No network policies in namespace 'default'
```

## Architecture

### Security Scanner Interface

```go
type SecurityScanner interface {
    // RBAC analysis
    AnalyzeRBAC(ctx context.Context, subject string) (*RBACAnalysis, error)
    CanI(ctx context.Context, subject, verb, resource, namespace string) (bool, error)
    
    // Policy scanning
    ListPolicies(ctx context.Context) ([]Policy, error)
    ListViolations(ctx context.Context) ([]Violation, error)
    
    // Image scanning
    ScanImages(ctx context.Context) ([]ImageScan, error)
    GetCVEs(ctx context.Context, image string) ([]CVE, error)
    
    // Network policies
    GetEffectiveNetPol(ctx context.Context, pod string) (*NetworkPolicy, error)
    
    // Overall score
    CalculateScore(ctx context.Context) (*SecurityScore, error)
}
```

### Integrations

| Feature | Backend |
|---------|---------|
| RBAC Analysis | Native K8s API |
| Policy Enforcement | OPA, Gatekeeper, Kyverno |
| CVE Scanning | Trivy, Grype |
| CIS Benchmarks | kube-bench |

## Configuration

```yaml
# ~/.config/kubecat/config.yaml
kubecat:
  security:
    enabled: true
    
    # CVE scanning
    imageScanner: trivy
    scanOnView: true
    
    # Policy providers
    policyProviders:
      - gatekeeper
      - kyverno
    
    # Thresholds for alerts
    thresholds:
      criticalCVEs: 0
      highCVEs: 5
      scoreMinimum: 70
```

