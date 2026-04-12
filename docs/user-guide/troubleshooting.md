# Troubleshooting (User Guide)

This is the user-facing quick reference. For detailed technical troubleshooting, see `docs/operations/troubleshooting.md`.

---

## "Cannot connect to cluster"

1. Verify kubectl works in Terminal: `kubectl get nodes`
2. Make sure you selected the right context from the cluster dropdown
3. For EKS: run `aws eks update-kubeconfig --name <cluster>` and try again
4. For GKE: run `gcloud container clusters get-credentials <cluster>` and try again

## AI is not responding

1. Open **Settings → AI** and verify your provider is enabled with a valid key
2. For Ollama: make sure Ollama is running — open Terminal and run `ollama list`
3. Check that the selected model is actually pulled: `ollama pull llama3.2`

## App shows blank screen

On macOS, open Terminal and check:
```bash
tail -50 ~/.local/state/kubecat/kubecat.log
```

Look for any errors about WebKit or rendering.

## I accidentally deleted a resource

Time Travel to the rescue: go to **Timeline**, select a snapshot from before the deletion, find the resource, and copy its YAML. Then re-apply it with kubectl or via the Resource Explorer.

## Cluster visualizer is empty

Make sure you've selected a namespace (or "All Namespaces") using the namespace selector at the top. The visualizer only shows resources in the active namespace by default.

## Update notification won't go away

Click the download link, update Kubecat, and relaunch. Update checks are disabled by default. To enable them, add `checkForUpdates: true` to your config.

## How do I reset everything?

```bash
# Remove all Kubecat data (config, history, logs, cache)
rm -rf ~/.config/kubecat ~/.local/state/kubecat ~/.cache/kubecat
```

Relaunch Kubecat and go through the onboarding flow again.
