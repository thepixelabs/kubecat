# Data Flow

> Applies to: Kubecat v0.x (2026). Last updated: 2026-04-07.

This document traces how data moves through the Kubecat Wails v2 + React + Go architecture for the four most important flows: cluster connection, resource loading, AI query, and event history collection.

**Audience:** Engineers debugging behavior or making architectural changes.

## IPC Model

Kubecat uses the Wails v2 binding model. Exported methods on the `App` struct are compiled into a TypeScript binding layer at build time. There is no HTTP API, no WebSocket, and no network port — all communication between the React frontend and the Go backend uses Wails IPC, which is an in-process call on macOS/Linux and a named pipe on Windows.

```
React (browser context)                Go (native context)
─────────────────────────────────────────────────────────
const result = await GetContexts()  →  func (a *App) GetContexts() ([]string, error)
                                    ←  return []string{"prod", "staging"}, nil

// Push from Go to React (no polling):
wails.EventsEmit(ctx, "cluster:health", payload)  →  useEvents("cluster:health", handler)
```

## 1. Cluster Connection Flow

```
User clicks context in UI
         │
         ▼
React: window.go.App.Connect("production")
         │  Wails IPC
         ▼
App.Connect(contextName string) error          [app.go]
  ├── checkReadOnly()                          // fails fast if readOnly: true
  ├── nexus.Clusters.Connect(ctx, contextName)
  │     ├── kubeconfig.ParseContext(contextName)
  │     ├── rest.Config from kubeconfig creds
  │     ├── dynamic.NewForConfig(restConfig)  // client-go dynamic client
  │     └── cluster added to manager pool
  ├── healthMonitor.NotifyConnected()
  └── go eventCollector.Refresh()             // starts event watch goroutine
         │
         ▼
React receives return value (nil error = success)
React state update → UI shows cluster as connected
         │
         ▼ (async, background)
EventCollector.watchCluster(ctx, "production")
  ├── clusterClient.List("events", limit=1000)  // initial sync
  └── clusterClient.Watch("events")             // streaming watch
```

## 2. Resource Loading Flow

```
User navigates to Explorer view, selects resource kind
         │
         ▼
React: window.go.App.ListResources(cluster, namespace, kind, options)
         │  Wails IPC
         ▼
App.ListResources(...)                         [app.go]
  └── nexus.Resources.List(ctx, cluster, namespace, kind, options)
        └── manager.Get(cluster)               // get ClusterClient from pool
              └── clusterClient.List(ctx, kind, ListOptions{Namespace, Limit})
                    └── dynamicClient.Resource(gvr).Namespace(ns).List(ctx, opts)
                          │  HTTP GET /apis/<group>/<version>/namespaces/<ns>/<resource>
                          ▼
                    Kubernetes API Server
                          │
                          ▼
                    UnstructuredList → []client.Resource (typed view)
         │
         ▼
[]Resource returned over Wails IPC as JSON
         │
         ▼
React: update table state → re-render resource list
```

GVR (Group/Version/Resource) lookups are cached in the client layer to minimize discovery API calls.

## 3. AI Query Flow

```
User types question, clicks "Ask AI"
         │
         ▼
React: window.go.App.QueryAI(question, namespace, stream=true)
         │  Wails IPC
         ▼
App.QueryAI(question, namespace string) (string, error)   [app.go]
  ├── ai.NewContextBuilder(clusterManager, eventRepo)
  │     .Build(ctx, question, namespace, providerName)
  │         ├── clusterClient.Info()                      // cluster name + version
  │         ├── inferResourceTypes(question, providerName) // keyword → resource kinds
  │         │     └── excludes "secrets" when providerName != "ollama"
  │         ├── clusterClient.List() for each inferred kind (limit 50 each)
  │         └── eventRepo.List(filter{Since: -1h, Limit: 50})
  │
  ├── ai.BuildPrompt(queryContext)
  │     ├── system prompt (Kubernetes expert persona)
  │     ├── cluster context block
  │     ├── resource list block
  │     ├── recent events block
  │     └── user question (wrapped in BEGIN/END delimiters — prompt injection mitigation)
  │
  ├── if isCloudProvider(providerName):
  │     ai.SanitizeForCloud(prompt)
  │       ├── redact Authorization: Bearer <token>
  │       ├── redact YAML/env values matching secret key patterns
  │       └── redact base64 blobs >64 chars
  │
  ├── network.Validate(endpoint, providerName)   // SSRF protection (5 layers)
  │
  └── provider.StreamQuery(ctx, prompt)
        │  HTTPS to AI provider API (or localhost for Ollama)
        ▼
  AI provider streams response tokens
        │  Wails EventsEmit("ai:token", token) per chunk
        ▼
React: accumulates tokens → renders streaming response
        │
        ▼
audit.Log(EventAIQuery, {cluster, namespace, provider, promptHash})
```

### Prompt injection mitigation

The user's question is wrapped in explicit `=== BEGIN USER QUESTION ===` / `=== END USER QUESTION ===` delimiters with an instruction telling the model to treat the content as data, not instructions. This prevents injected text like "Ignore previous instructions..." from being processed as system directives.

## 4. Event History Collection Flow (background)

This flow runs entirely in Go goroutines. The React frontend never directly initiates it.

```
App.startup(ctx)
  └── eventCollector.Start()
        └── go run()
              ├── refreshWatches()
              │     └── for each connected cluster:
              │           go watchCluster(ctx, clusterName)
              │                 ├── syncExistingEvents()    // LIST /api/v1/events
              │                 └── Watch("events")         // streaming WATCH
              │                       for event := range eventCh:
              │                         processEvent(ctx, clusterName, event)
              │                           ├── parseK8sEvent(resource)
              │                           ├── repo.Save(ctx, storedEvent)  → SQLite
              │                           └── correlator.CorrelateEvent()  → correlations table
              │
              └── every 1 hour: refreshWatches() + cleanup(cutoff = now - 7d)

              // If Watch channel closes (API disconnect):
              // exponential backoff 1s → 2s → 4s ... → 60s cap, retry indefinitely
```

### Snapshotting flow (background)

```
App.startup(ctx)
  └── snapshotter.Start()
        └── go run()
              ├── takeSnapshot() immediately on start
              └── every 5 minutes:
                    takeSnapshot()
                      └── for each connected cluster:
                            snapshotCluster(ctx, clusterName)
                              ├── clusterClient.List("namespaces")
                              ├── for each kind in [pods, deployments, services,
                              │         configmaps, statefulsets, daemonsets,
                              │         jobs, cronjobs]:
                              │     clusterClient.List(kind, limit=10000)
                              └── repo.Save(ctx, clusterName, snapshotData)
                                    → JSON-encoded blob → zlib-compressed → SQLite
                    cleanup(cutoff = now - 7d)
```

## 5. Wails Event Push (Go → React)

The `events.Emitter` calls `wails.EventsEmit()` to push notifications to the React frontend without polling. The frontend subscribes with `useWailsEvent()` hooks.

| Wails event name | Emitted when | Frontend action |
|-----------------|-------------|----------------|
| `cluster:health` | Health monitor state change | Update cluster status badge |
| `cluster:connected` | Cluster reconnects | Refresh resource lists |
| `cluster:disconnected` | Connection lost | Show disconnected state |
| `ai:token` | AI response chunk arrives | Append to streaming response |
| `alert:fired` | Proactive alert triggered | Show alert banner |
| `update:available` | New release detected | Show update notification bar |

## 6. Agent (Agentic AI) Flow

When the user enables the Agent mode, the flow extends the AI query flow with an observe-think-act loop:

```
App.RunAgentSession(question, namespace)
  └── Agent{provider, guardrails, emitter, executor}.Run(ctx, question)
        ├── iteration 1..10 (maxIterations = 10):
        │     ├── provider.Query(ctx, systemPrompt + conversation)
        │     │     // LLM returns either a final answer or a tool call JSON block
        │     ├── parse response for ToolCall{Name, Parameters}
        │     ├── guardrails.CheckTool(toolName, namespace, cluster, tokensUsed)
        │     │     ├── layer 1: tool must exist in registry
        │     │     ├── layer 2: namespace allowlist check
        │     │     ├── layer 3: protected namespace block
        │     │     ├── layer 4: destructive tool block (if enabled)
        │     │     ├── layer 5: double-confirm gate for destructive tools
        │     │     ├── layer 6: session rate limit (default 20 calls/60s)
        │     │     ├── layer 7: session tool cap (default 50 total)
        │     │     └── token budget check (default 100,000 tokens)
        │     ├── if !allowed: emit tool_blocked event → frontend shows warning
        │     ├── executor.ExecuteTool(ctx, toolName, params)
        │     │     // Executes read/write K8s operation on behalf of the agent
        │     ├── emit agentToolResult event → frontend shows tool result
        │     └── append tool result to conversation history
        └── emit agentComplete event → frontend renders final answer
```

## Data Never Leaves The Host Unless...

- The user explicitly configures a cloud AI provider (OpenAI, Anthropic, Google).
- With a cloud provider, only sanitized prompt text is sent — no kubeconfig credentials, no raw secret values, no base64 blobs.
- Anonymous telemetry is opt-in and transmits no cluster data (see `internal/telemetry`).
- The auto-update checker queries GitHub releases API for version metadata only (no cluster data).

## Related Documents

- [Architecture Overview](overview.md)
- [AI System](ai-system.md)
- [Storage](storage.md)
- [History](history.md)
- [Security: Data Handling](../security/data-handling.md)
