# Port Forwarding

Kubecat provides one-click port forwarding from a pod to your localhost, equivalent to `kubectl port-forward`.

---

## Creating a Port Forward

1. Open the **Resource Explorer** and navigate to a Pod.
2. In the pod detail panel, click **Port Forward** next to any container port.
3. Kubecat assigns a local port (or you can specify one) and starts forwarding.

Alternatively, from the **Port Forwards** view in the sidebar, click **New Port Forward** and fill in:
- Cluster
- Namespace
- Pod name
- Remote port (the container port)
- Local port (defaults to the same as remote)

---

## Managing Port Forwards

The **Port Forwards** view lists all active forwards:

| Field | Description |
|-------|-------------|
| Pod | `namespace/pod-name` |
| Local port | `localhost:PORT` |
| Remote port | Container port |
| Status | Active / Error |

Click **Open in Browser** to open `http://localhost:PORT` in your default browser (for HTTP services).

Click **Stop** to terminate a port forward.

---

## Accessing the Forwarded Service

Once a port forward is active, access the service at:

```
http://localhost:<local-port>
```

or

```
tcp://localhost:<local-port>
```

for non-HTTP protocols (databases, gRPC, etc.).

---

## Limitations

- Port forwards bind to a specific pod instance. If the pod restarts (new pod name), the forward breaks and must be recreated.
- Port forwards are terminated when Kubecat is closed.
- Service-level port forwarding (following pod restarts automatically) is a planned future feature.
- Port forwarding is blocked when the cluster is in `readOnly` mode.

---

## Troubleshooting

If a port forward shows "Error" status:

1. Verify the pod is still running: check the Resource Explorer.
2. Verify the container port is correct.
3. Check if another process is already using the local port: `lsof -i :<port>`.
4. Check the application log for connection errors:
   ```bash
   grep -i "portforward" ~/.local/state/kubecat/kubecat.log | tail -20
   ```
