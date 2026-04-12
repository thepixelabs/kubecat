// SPDX-License-Identifier: Apache-2.0

// Package updater checks GitHub Releases for newer versions of Kubecat and
// emits a Wails event when one is found.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thepixelabs/kubecat/internal/events"
	"github.com/thepixelabs/kubecat/internal/version"
)

const (
	githubReleasesURL = "https://api.github.com/repos/kubecat/kubecat/releases/latest"
	checkInterval     = 24 * time.Hour
	startupDelay      = 10 * time.Second
	httpTimeout       = 15 * time.Second
)

// UpdateAvailableEvent is the payload emitted on "app:update-available".
type UpdateAvailableEvent struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseURL     string `json:"releaseUrl"`
	ReleaseNotes   string `json:"releaseNotes"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
}

// Updater periodically polls GitHub Releases and emits update events.
type Updater struct {
	emitter events.EmitterInterface
	client  *http.Client

	mu         sync.Mutex
	lastCheck  time.Time
	cancelFunc context.CancelFunc
	done       chan struct{}
}

// New creates an Updater. Call Start to begin polling.
func New(em events.EmitterInterface) *Updater {
	return &Updater{
		emitter: em,
		client:  &http.Client{Timeout: httpTimeout},
		done:    make(chan struct{}),
	}
}

// Start begins the update check loop after an initial startup delay.
func (u *Updater) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	u.cancelFunc = cancel
	go u.loop(ctx)
}

// Stop shuts down the update check loop.
func (u *Updater) Stop() {
	if u.cancelFunc != nil {
		u.cancelFunc()
	}
	<-u.done
}

func (u *Updater) loop(ctx context.Context) {
	defer close(u.done)

	// Wait for startup delay before the first check so we don't slow down boot.
	select {
	case <-ctx.Done():
		return
	case <-time.After(startupDelay):
	}

	u.check(ctx)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.check(ctx)
		}
	}
}

// check fetches the latest release from GitHub and emits an event if a newer
// version is available.  The 24h rate limit prevents hammering the API.
func (u *Updater) check(ctx context.Context) {
	u.mu.Lock()
	if !u.lastCheck.IsZero() && time.Since(u.lastCheck) < checkInterval {
		u.mu.Unlock()
		return
	}
	u.mu.Unlock()

	latest, err := u.fetchLatestRelease(ctx)
	if err != nil {
		slog.Debug("updater: failed to fetch latest release", slog.Any("error", err))
		return
	}

	u.mu.Lock()
	u.lastCheck = time.Now()
	u.mu.Unlock()

	current := version.Version
	if !isNewer(latest.TagName, current) {
		slog.Debug("updater: already on latest version", slog.String("version", current))
		return
	}

	slog.Info("updater: new version available",
		slog.String("current", current),
		slog.String("latest", latest.TagName))

	u.emitter.Emit("app:update-available", UpdateAvailableEvent{
		CurrentVersion: current,
		LatestVersion:  latest.TagName,
		ReleaseURL:     latest.HTMLURL,
		ReleaseNotes:   latest.Body,
	})
}

func (u *Updater) fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kubecat/"+version.Version)

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

// isNewer returns true when latestTag is a semver higher than currentVersion.
// Both versions may optionally have a "v" prefix.
func isNewer(latestTag, currentVersion string) bool {
	latest := normalizeSemver(latestTag)
	current := normalizeSemver(currentVersion)

	if latest == "" || current == "" || latest == current {
		return false
	}

	lp := strings.SplitN(latest, ".", 3)
	cp := strings.SplitN(current, ".", 3)

	for i := 0; i < 3; i++ {
		lv := semverPart(lp, i)
		cv := semverPart(cp, i)
		if lv > cv {
			return true
		}
		if lv < cv {
			return false
		}
	}
	return false
}

func normalizeSemver(v string) string {
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release or build metadata.
	if idx := strings.IndexAny(v, "-+"); idx >= 0 {
		v = v[:idx]
	}
	return v
}

func semverPart(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n := 0
	for _, c := range parts[i] {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
