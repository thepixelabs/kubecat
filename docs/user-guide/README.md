# Kubecat User Guide

Welcome to Kubecat — a Kubernetes management desktop application built for engineers who live in the terminal but want a smarter cockpit view.

---

## What Kubecat Does

Kubecat connects directly to your Kubernetes clusters using your existing kubeconfig. It provides:

- **Resource Explorer** — browse and manage any Kubernetes resource across namespaces
- **Cluster Visualizer** — interactive graph showing relationships between resources
- **AI Queries** — ask questions about your cluster in plain English
- **Security Scanning** — RBAC, runtime, and network policy analysis
- **Time Travel** — snapshot-based history to see what changed and when
- **Cluster Diff** — compare two clusters or two points in time
- **GitOps Integration** — ArgoCD and Flux status in one view
- **Terminal** — embedded shell with kubectl access
- **Port Forwarding** — one-click pod port forwarding

---

## Guide Contents

| Topic | Description |
|-------|-------------|
| [Getting Started](getting-started.md) | Installation, first launch, connecting your cluster |
| [Cluster Management](cluster-management.md) | Multiple clusters, contexts, read-only mode |
| [Resource Explorer](resource-explorer.md) | Browsing, filtering, and editing resources |
| [AI Queries](ai-queries.md) | Asking questions, configuring providers, safety |
| [Security Scanning](security-scanning.md) | RBAC analysis, runtime scanning, policy engines |
| [Time Travel](time-travel.md) | Snapshots, event timeline, historical state |
| [Cluster Diff](cluster-diff.md) | Comparing clusters and snapshots |
| [GitOps](gitops.md) | ArgoCD and Flux integration |
| [Terminal](terminal.md) | Embedded shell, kubectl commands |
| [Port Forwarding](port-forwarding.md) | Forwarding pod ports to localhost |
| [Configuration](configuration.md) | Full config file reference |
| [Troubleshooting](troubleshooting.md) | Common issues and fixes |

---

## Quick Start

1. [Install Kubecat](getting-started.md#installation) (macOS)
2. Launch the app — it auto-detects your kubeconfig
3. Select a cluster context and click **Connect**
4. Explore resources, run an AI query, or view the cluster visualizer

---

## Design Philosophy

Kubecat is **local-first**. All your data stays on your machine. Cluster credentials never leave your workstation. AI queries only transmit the specific resource data you ask about, and only to the provider you configure. Ollama support means zero data leaves your network.

Kubecat is **read-only by default** on a per-cluster basis. Enable write operations explicitly in config for each cluster you want to manage.
