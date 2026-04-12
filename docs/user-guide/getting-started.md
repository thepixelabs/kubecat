# Getting Started

## Installation

### Homebrew (recommended)

```bash
brew install --cask thepixelabs/thepixelabs/kubecat
```

### Manual

1. Download the latest `.dmg` from the [GitHub Releases page](https://github.com/thepixelabs/kubecat/releases).
2. Open the DMG and drag **Kubecat.app** to your Applications folder.
3. Launch Kubecat.

**First launch on macOS:** If you see "Kubecat can't be opened because it's from an unidentified developer", go to **System Settings → Privacy & Security** and click **Open Anyway** next to the Kubecat entry.

### Prerequisites

- macOS 12 Monterey or later (macOS 13+ recommended)
- A valid `~/.kube/config` with at least one cluster context
- For AI features: either [Ollama](https://ollama.ai) running locally, or an API key for OpenAI, Anthropic, or Google

---

## First Launch

When you open Kubecat for the first time, the onboarding wizard walks you through:

1. **Welcome** — brief overview of Kubecat's capabilities.
2. **Connect a cluster** — Kubecat auto-discovers your kubeconfig contexts. Click any context card to connect.
3. **AI setup** — choose your AI provider. Ollama (local, free, private) is recommended as a starting point.
4. **Quick tour** — a 4-step overlay highlights the main features.

---

## Connecting to a Cluster

After the initial setup, use the **cluster selector** in the Navbar (top bar) to:

- Switch between connected clusters
- Connect to additional clusters
- Disconnect from clusters you are not using

Kubecat reads your kubeconfig from `~/.kube/config` by default. To use a different kubeconfig, set the `KUBECONFIG` environment variable before launching the app.

---

## What Gets Connected

When you connect to a cluster, Kubecat:

1. Establishes a connection using your kubeconfig credentials (same as `kubectl`).
2. Starts the event collector — continuously ingesting Kubernetes events into the local history database.
3. Takes an initial cluster snapshot for the time-travel baseline.
4. Begins a 30-second heartbeat check to monitor connection health.

No persistent connection is maintained beyond what is needed for each operation. Kubecat is not a continuously-polling daemon — it fetches on demand and receives events via Kubernetes Watch.

---

## Updating Kubecat

Kubecat checks for updates once per day. When a new version is available, a notification bar appears at the top of the window with a link to the release page.

To update via Homebrew:

```bash
brew upgrade --cask kubecat
```

To disable update checks, see the [Configuration](configuration.md) guide.
