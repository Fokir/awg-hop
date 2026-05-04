CREATE TABLE IF NOT EXISTS system_settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	external_interface TEXT NOT NULL DEFAULT '',
	tunnel_offline_policy TEXT NOT NULL DEFAULT 'block' CHECK (tunnel_offline_policy IN ('block', 'ignore')),
	client_allowed_ipv4 TEXT NOT NULL DEFAULT '0.0.0.0/0',
	client_allowed_ipv6 TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
);

INSERT OR IGNORE INTO system_settings (id, external_interface, tunnel_offline_policy, client_allowed_ipv4, client_allowed_ipv6, updated_at)
VALUES (1, '', 'block', '0.0.0.0/0', '', strftime('%Y-%m-%dT%H:%M:%SZ','now'));

CREATE INDEX IF NOT EXISTS idx_egress_iface_name ON egress_tunnels (interface_name);
