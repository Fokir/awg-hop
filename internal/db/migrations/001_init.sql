-- AWG Hop initial schema.
--
-- Терминология:
--   * clients          — устройства конечных пользователей (аналог wg-easy clients).
--   * upstream_tunnels — наши исходящие AmneziaWG-подключения, в которых наш
--                        сервер выступает клиентом другого AWG-сервера.

CREATE TABLE IF NOT EXISTS admin_account (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	password_hash TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ingress_settings (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	listen_port INTEGER NOT NULL DEFAULT 51820,
	host_endpoint TEXT NOT NULL DEFAULT '',
	tunnel_subnet TEXT NOT NULL DEFAULT '10.8.0.0/24',
	dns_servers TEXT NOT NULL DEFAULT '1.1.1.1',
	mtu INTEGER NOT NULL DEFAULT 1420,
	interface_name TEXT NOT NULL DEFAULT 'awg0',
	server_tunnel_ip TEXT NOT NULL DEFAULT '10.8.0.1',
	jc INTEGER NOT NULL DEFAULT 4,
	jmin INTEGER NOT NULL DEFAULT 50,
	jmax INTEGER NOT NULL DEFAULT 1000,
	s1 TEXT NOT NULL DEFAULT '',
	s2 TEXT NOT NULL DEFAULT '',
	s3 TEXT NOT NULL DEFAULT '',
	s4 TEXT NOT NULL DEFAULT '',
	h1 INTEGER NOT NULL DEFAULT 0,
	h2 INTEGER NOT NULL DEFAULT 0,
	h3 INTEGER NOT NULL DEFAULT 0,
	h4 INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
	token TEXT PRIMARY KEY,
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS clients (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	private_key TEXT NOT NULL,
	public_key TEXT NOT NULL,
	preshared_key TEXT,
	allowed_ips TEXT NOT NULL,
	enabled INTEGER NOT NULL DEFAULT 1,
	upstream_type TEXT NOT NULL DEFAULT 'direct' CHECK (upstream_type IN ('direct', 'via_upstream')),
	upstream_tunnel_id INTEGER,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_clients_enabled ON clients (enabled);
