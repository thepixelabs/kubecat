# Terminal

Kubecat includes an embedded terminal with a full PTY (pseudo-terminal) implementation, giving you a shell session with access to your local tools including `kubectl`.

---

## Opening the Terminal

Click **Terminal** in the sidebar, or press the expand button in the terminal drawer at the bottom of the screen.

The terminal opens a shell session on your local machine — the same as opening Terminal.app. Your `PATH`, environment variables, and shell configuration (`~/.zshrc`, `~/.bashrc`) are inherited.

---

## Using kubectl in the Terminal

Since the terminal runs a local shell, `kubectl` commands use your current `KUBECONFIG` and context. Kubecat does not automatically switch the kubectl context to match the cluster selected in the UI — use `kubectl config use-context <name>` or the `--context` flag explicitly.

```bash
kubectl get pods --context my-cluster --namespace default
```

---

## Shell Support

The terminal restricts shell selection to:
- `bash`
- `zsh`
- `sh`

This is a security control — arbitrary process execution is not permitted through the terminal RPC.

---

## Terminal and Read-Only Mode

When the cluster is configured with `readOnly: true`, the terminal is blocked. This is intentional: the terminal provides full local shell access, which could be used to run `kubectl delete` or other mutating commands regardless of Kubecat's read-only flag.

---

## Resize

The terminal automatically resizes when you resize the window or expand/collapse the terminal drawer. The PTY resize signal (SIGWINCH) is forwarded correctly.

---

## Session Lifetime

The terminal session persists as long as the shell process is running. Closing the terminal drawer does not kill the shell — it remains in the background. You can reopen the drawer to resume the same session.

Sessions are terminated when Kubecat is closed (graceful shutdown sends SIGHUP to all PTY processes).
