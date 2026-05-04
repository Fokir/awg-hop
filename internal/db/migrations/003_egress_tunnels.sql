CREATE TABLE IF NOT EXISTS egress_tunnels (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	interface_name TEXT NOT NULL UNIQUE,
	config_text TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_egress_enabled ON egress_tunnels (enabled);
