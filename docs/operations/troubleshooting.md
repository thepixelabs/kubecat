# Troubleshooting

Common issues and how to resolve them.

---

## 1. "Cannot connect to cluster" / kubeconfig not found

**Symptom:** Kubecat shows no clusters in the context dropdown, or shows "connection failed".

**Causes and fixes:**
- Kubecat reads `~/.kube/config` by default. Verify it exists: `ls -la ~/.kube/config`
- If using a non-default kubeconfig, set `KUBECONFIG=/path/to/your/config` in your shell profile and relaunch Kubecat.
- Verify your kubeconfig is valid: `kubectl get nodes` (if this fails, the problem is with your kubeconfig, not Kubecat).
- For EKS clusters, ensure `aws-iam-authenticator` or `aws` CLI is on your PATH and authenticated.
- For GKE clusters, run `gcloud container clusters get-credentials <name>` to refresh credentials.

---

## 2. App opens but shows blank white screen

**Symptom:** The Wails window opens but shows nothing, or shows a white rectangle.

**Causes and fixes:**
- On macOS, this often means WebKit failed to load. Check the application log:
  ```bash
  tail -100 ~/.local/state/kubecat/kubecat.log
  ```
- Try resetting the WebKit cache:
  ```bash
  rm -rf ~/Library/Caches/kubecat
  ```
- On macOS 15+, Gatekeeper may block the app due to ad-hoc signing. In **System Settings → Privacy & Security**, scroll down to find the blocked app and click "Open Anyway".

---

## 3. AI queries fail with "provider not configured"

**Symptom:** AI queries return an error about no provider being configured.

**Fixes:**
- Open **Settings → AI** and verify a provider is enabled with a valid API key.
- For Ollama: verify Ollama is running (`ollama list`) and the endpoint is reachable (`curl http://localhost:11434/api/tags`).
- For cloud providers: verify your API key is valid by testing it directly with `curl`.
- Check the config file for syntax errors: `cat ~/.config/kubecat/config.yaml`

---

## 4. AI responses appear blank or cut off

**Symptom:** The AI response area shows a spinner that never resolves, or shows partial text.

**Causes and fixes:**
- Streaming SSE responses are buffered. Check your provider's rate limits and quota.
- For Ollama: the model may still be loading. Watch `ollama ps` for the model to appear.
- Check the log for streaming errors:
  ```bash
  grep -i "ai\|stream\|provider" ~/.local/state/kubecat/kubecat.log | tail -50
  ```

---

## 5. Cluster visualizer is empty or shows "layout failed"

**Symptom:** The cluster visualization view shows no nodes, or shows an error banner.

**Fixes:**
- Switch namespace using the namespace selector — the visualizer only shows resources in the selected namespace.
- Select "All namespaces" (`__all__`) to see the full cluster graph.
- Large clusters (1,000+ nodes) may cause ELK layout to time out. Try filtering to a specific namespace.
- If the error persists, open the browser console (right-click → Inspect) and look for JavaScript errors.

---

## 6. Terminal does not open / shows no output

**Symptom:** Clicking the Terminal tab shows a blank area or the terminal immediately closes.

**Fixes:**
- Ensure `bash` or `zsh` is available on your PATH.
- Check the application log for PTY errors:
  ```bash
  grep -i "terminal\|pty" ~/.local/state/kubecat/kubecat.log | tail -20
  ```
- On macOS, the app may need Terminal access permission. Check **System Settings → Privacy & Security → Full Disk Access** (not usually needed) or try relaunching.

---

## 7. Port forwarding stops working after a few minutes

**Symptom:** Port forwards work initially but TCP connections start refusing after some time.

**Causes and fixes:**
- The target pod may have been restarted. Port forwards bind to a specific pod instance — they do not follow deployments automatically.
- Stop the port forward and create a new one after the pod restarts.
- Future improvement: Kubecat will support service-level port forwarding that follows pod restarts.

---

## 8. History / timeline shows no events

**Symptom:** The timeline view shows no events even after connecting to a cluster with known activity.

**Fixes:**
- Event collection starts when you first connect to a cluster. If you just connected, wait 30–60 seconds.
- Verify the SQLite database is being written to:
  ```bash
  sqlite3 ~/.local/state/kubecat/history.db "SELECT COUNT(*) FROM events;"
  ```
- If count is 0, check for permission errors in the log:
  ```bash
  grep -i "storage\|database\|sqlite" ~/.local/state/kubecat/kubecat.log | tail -30
  ```

---

## 9. "Read-only mode" error when trying to delete or apply resources

**Symptom:** Operations like Delete or Apply fail with a "read-only mode" error.

**Fix:**
Edit `~/.config/kubecat/config.yaml` and set:

```yaml
kubecat:
  readOnly: false
```

Or for a specific cluster only:

```yaml
kubecat:
  clusters:
    - context: my-prod-cluster
      readOnly: false
```

Restart Kubecat after changing the config.

---

## Collecting Diagnostics for Bug Reports

When filing a bug report, include:

1. Kubecat version (shown in **Settings → About**)
2. macOS version (`sw_vers`)
3. Last 200 lines of the application log:
   ```bash
   tail -200 ~/.local/state/kubecat/kubecat.log
   ```
4. Steps to reproduce

Do **not** include your kubeconfig or API keys in bug reports.
