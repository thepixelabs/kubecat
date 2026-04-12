# Auto-Update

Kubecat checks for new releases via the GitHub Releases API and notifies users non-intrusively.

---

## How It Works

On application startup, a background goroutine checks for a newer version of Kubecat. The check:

1. Reads the current version from `internal/version.Version` (set at build time via ldflags).
2. Fetches `https://api.github.com/repos/thepixelabs/kubecat/releases/latest`.
3. Compares the `tag_name` from the API response against the running version using semantic versioning.
4. If a newer version is available, emits the Wails event `app:update-available` to the frontend.
5. The frontend displays a non-intrusive notification bar: **"Kubecat vX.Y.Z is available — Download"**.

---

## Rate Limiting

To respect GitHub API rate limits and avoid noise:

- The check runs **once per 24 hours** at most.
- The timestamp of the last check is stored in the settings table of the local SQLite database.
- If the check fails (network error, rate limit response), it is silently skipped — no error is shown to the user.
- The check uses the unauthenticated GitHub API (60 requests/hour per IP), which is sufficient for a once-daily check.

---

## Opting In

Update checks are disabled by default. Set `checkForUpdates: true` in `~/.config/kubecat/config.yaml` to enable them:

```yaml
kubecat:
  checkForUpdates: true
```

When enabled, Kubecat checks the GitHub Releases API on startup and notifies you of new versions.

---

## Manual Check

There is no manual "Check for updates" button in the UI yet. To manually check:

1. Visit https://github.com/thepixelabs/kubecat/releases
2. Compare against the version shown in **Settings → About**.

---

## Dev Builds

Development builds (`Version = "0.1.0-dev"`) skip the update check entirely — the version string cannot be compared against release tags.
