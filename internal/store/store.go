package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"awghop/internal/domain"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// DB возвращает текущий *sql.DB; используется кодом, которому нужна работа с
// низкоуровневыми пакетами (например, backup-export читает schema_migrations).
func (s *Store) DB() *sql.DB { return s.db }

// SwapDB атомарно подменяет соединение. Закрывает старое соединение.
// Допускает короткий интервал, когда параллельные запросы могут получить ошибку —
// используется только из административного backup-import.
func (s *Store) SwapDB(newDB *sql.DB) {
	old := s.db
	s.db = newDB
	if old != nil {
		_ = old.Close()
	}
}

func (s *Store) SetupComplete(ctx context.Context) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_account`).Scan(&n)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return n > 0, nil
}

func (s *Store) Bootstrap(ctx context.Context, passwordHash string, in domain.IngressSettings, serverPriv, serverPub string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `INSERT INTO admin_account (id, password_hash, created_at) VALUES (1, ?, ?)`,
		passwordHash, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
	INSERT INTO ingress_settings (
		id, listen_port, host_endpoint, tunnel_subnet, dns_servers, mtu, interface_name, server_tunnel_ip,
		jc, jmin, jmax, s1, s2, s3, s4, h1, h2, h3, h4, updated_at,
		server_private_key, server_public_key
	) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.ListenPort, in.HostEndpoint, in.TunnelSubnet, in.DNSServers, in.MTU, in.InterfaceName, in.ServerTunnelIP,
		in.Jc, in.Jmin, in.Jmax, in.S1, in.S2, in.S3, in.S4, in.H1, in.H2, in.H3, in.H4, now,
		serverPriv, serverPub,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetPasswordHash(ctx context.Context) (string, error) {
	var h string
	err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM admin_account WHERE id = 1`).Scan(&h)
	return h, err
}

func (s *Store) CreateSession(ctx context.Context, token string, expiresAt time.Time) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token, expires_at, created_at) VALUES (?, ?, ?)`,
		token, expiresAt.UTC().Format(time.RFC3339), now)
	return err
}

func (s *Store) SessionValid(ctx context.Context, token string) (bool, error) {
	var exp string
	err := s.db.QueryRowContext(ctx, `SELECT expires_at FROM sessions WHERE token = ?`, token).Scan(&exp)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	t, err := time.Parse(time.RFC3339, exp)
	if err != nil {
		return false, nil
	}
	return time.Now().UTC().Before(t), nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func (s *Store) GetIngressSettings(ctx context.Context) (domain.IngressSettings, error) {
	var in domain.IngressSettings
	var h1, h2, h3, h4 sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
	SELECT listen_port, host_endpoint, tunnel_subnet, dns_servers, mtu, interface_name, server_tunnel_ip,
	       jc, jmin, jmax, s1, s2, s3, s4, h1, h2, h3, h4,
	       server_private_key, server_public_key
	FROM ingress_settings WHERE id = 1`).Scan(
		&in.ListenPort, &in.HostEndpoint, &in.TunnelSubnet, &in.DNSServers, &in.MTU, &in.InterfaceName, &in.ServerTunnelIP,
		&in.Jc, &in.Jmin, &in.Jmax, &in.S1, &in.S2, &in.S3, &in.S4, &h1, &h2, &h3, &h4,
		&in.ServerPrivateKey, &in.ServerPublicKey,
	)
	if h1.Valid {
		in.H1 = h1.Int64
	}
	if h2.Valid {
		in.H2 = h2.Int64
	}
	if h3.Valid {
		in.H3 = h3.Int64
	}
	if h4.Valid {
		in.H4 = h4.Int64
	}
	return in, err
}

func (s *Store) UpdateIngressSettings(ctx context.Context, in domain.IngressSettings) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
	UPDATE ingress_settings SET
		listen_port = ?, host_endpoint = ?, tunnel_subnet = ?, dns_servers = ?, mtu = ?, interface_name = ?, server_tunnel_ip = ?,
		jc = ?, jmin = ?, jmax = ?, s1 = ?, s2 = ?, s3 = ?, s4 = ?, h1 = ?, h2 = ?, h3 = ?, h4 = ?, updated_at = ?,
		server_private_key = ?, server_public_key = ?
	WHERE id = 1`,
		in.ListenPort, in.HostEndpoint, in.TunnelSubnet, in.DNSServers, in.MTU, in.InterfaceName, in.ServerTunnelIP,
		in.Jc, in.Jmin, in.Jmax, in.S1, in.S2, in.S3, in.S4, in.H1, in.H2, in.H3, in.H4, now,
		in.ServerPrivateKey, in.ServerPublicKey,
	)
	return err
}

func (s *Store) ListClientAllowedIPs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT allowed_ips FROM clients`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) ListClients(ctx context.Context) ([]domain.Client, error) {
	rows, err := s.db.QueryContext(ctx, `
	SELECT id, name, public_key, private_key, preshared_key, allowed_ips, enabled, upstream_type, upstream_tunnel_id, created_at, updated_at
	FROM clients ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.Client
	for rows.Next() {
		c, err := scanClient(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

func scanClient(sc interface {
	Scan(dest ...any) error
}) (domain.Client, error) {
	var c domain.Client
	var psk sql.NullString
	var upstream sql.NullInt64
	var created, updated string
	err := sc.Scan(&c.ID, &c.Name, &c.PublicKey, &c.PrivateKey, &psk, &c.AllowedIPs, &c.Enabled, &c.UpstreamType, &upstream, &created, &updated)
	if err != nil {
		return c, err
	}
	if psk.Valid {
		s := psk.String
		c.PresharedKey = &s
	}
	if upstream.Valid {
		v := upstream.Int64
		c.UpstreamTunnelID = &v
	}
	if t, err := time.Parse(time.RFC3339, created); err == nil {
		c.CreatedAt = t
	}
	if t, err := time.Parse(time.RFC3339, updated); err == nil {
		c.UpdatedAt = t
	}
	return c, nil
}

func (s *Store) GetClient(ctx context.Context, id int64) (domain.Client, error) {
	row := s.db.QueryRowContext(ctx, `
	SELECT id, name, public_key, private_key, preshared_key, allowed_ips, enabled, upstream_type, upstream_tunnel_id, created_at, updated_at
	FROM clients WHERE id = ?`, id)
	return scanClient(row)
}

func (s *Store) InsertClient(ctx context.Context, c domain.Client) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var psk any
	if c.PresharedKey != nil {
		psk = *c.PresharedKey
	}
	var upstream any
	if c.UpstreamTunnelID != nil {
		upstream = *c.UpstreamTunnelID
	}
	res, err := s.db.ExecContext(ctx, `
	INSERT INTO clients (name, private_key, public_key, preshared_key, allowed_ips, enabled, upstream_type, upstream_tunnel_id, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Name, c.PrivateKey, c.PublicKey, psk, c.AllowedIPs, c.Enabled, string(c.UpstreamType), upstream, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateClient(ctx context.Context, c domain.Client) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var psk any
	if c.PresharedKey != nil {
		psk = *c.PresharedKey
	}
	var upstream any
	if c.UpstreamTunnelID != nil {
		upstream = *c.UpstreamTunnelID
	}
	_, err := s.db.ExecContext(ctx, `
	UPDATE clients SET name = ?, private_key = ?, public_key = ?, preshared_key = ?, allowed_ips = ?, enabled = ?, upstream_type = ?, upstream_tunnel_id = ?, updated_at = ?
	WHERE id = ?`,
		c.Name, c.PrivateKey, c.PublicKey, psk, c.AllowedIPs, c.Enabled, string(c.UpstreamType), upstream, now, c.ID,
	)
	return err
}

func (s *Store) DeleteClient(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM clients WHERE id = ?`, id)
	return err
}
