// SPDX-License-Identifier: Apache-2.0

package storage

// Migration represents a database migration.
type Migration struct {
	Version int
	SQL     string
}

// migrations is the list of database migrations.
var migrations = []Migration{
	{
		Version: 1,
		SQL: `
			-- Snapshots table stores periodic cluster state snapshots
			CREATE TABLE IF NOT EXISTS snapshots (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				cluster TEXT NOT NULL,
				timestamp DATETIME NOT NULL,
				data BLOB NOT NULL,
				compressed INTEGER DEFAULT 1,
				UNIQUE(cluster, timestamp)
			);
			CREATE INDEX IF NOT EXISTS idx_snapshots_cluster ON snapshots(cluster);
			CREATE INDEX IF NOT EXISTS idx_snapshots_timestamp ON snapshots(timestamp);

			-- Events table stores Kubernetes events
			CREATE TABLE IF NOT EXISTS events (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				cluster TEXT NOT NULL,
				namespace TEXT,
				kind TEXT NOT NULL,
				name TEXT NOT NULL,
				type TEXT NOT NULL,
				reason TEXT,
				message TEXT,
				first_seen DATETIME,
				last_seen DATETIME NOT NULL,
				count INTEGER DEFAULT 1,
				source_component TEXT,
				source_host TEXT
			);
			CREATE INDEX IF NOT EXISTS idx_events_cluster ON events(cluster);
			CREATE INDEX IF NOT EXISTS idx_events_namespace ON events(namespace);
			CREATE INDEX IF NOT EXISTS idx_events_kind_name ON events(kind, name);
			CREATE INDEX IF NOT EXISTS idx_events_last_seen ON events(last_seen);
			CREATE INDEX IF NOT EXISTS idx_events_reason ON events(reason);

			-- Correlations table links related events
			CREATE TABLE IF NOT EXISTS correlations (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				source_event_id INTEGER NOT NULL,
				target_event_id INTEGER NOT NULL,
				confidence REAL NOT NULL,
				relationship TEXT NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (source_event_id) REFERENCES events(id) ON DELETE CASCADE,
				FOREIGN KEY (target_event_id) REFERENCES events(id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_correlations_source ON correlations(source_event_id);
			CREATE INDEX IF NOT EXISTS idx_correlations_target ON correlations(target_event_id);

			-- Resources table tracks resource versions for change detection
			CREATE TABLE IF NOT EXISTS resources (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				cluster TEXT NOT NULL,
				namespace TEXT,
				kind TEXT NOT NULL,
				name TEXT NOT NULL,
				resource_version TEXT NOT NULL,
				data BLOB,
				first_seen DATETIME NOT NULL,
				last_seen DATETIME NOT NULL,
				UNIQUE(cluster, namespace, kind, name)
			);
			CREATE INDEX IF NOT EXISTS idx_resources_cluster ON resources(cluster);
			CREATE INDEX IF NOT EXISTS idx_resources_kind ON resources(kind);
		`,
	},
	{
		Version: 2,
		SQL: `
			-- Settings table for user preferences
			CREATE TABLE IF NOT EXISTS settings (
				key TEXT PRIMARY KEY,
				value TEXT NOT NULL,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
		`,
	},
}
