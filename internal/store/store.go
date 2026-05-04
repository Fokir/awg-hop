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

func (s *Store) ListPeerAllowedIPs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT allowed_ips FROM peers`)
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

func (s *Store) ListPeers(ctx context.Context) ([]domain.Peer, error) {
	rows, err := s.db.QueryContext(ctx, `
	SELECT id, name, public_key, private_key, preshared_key, allowed_ips, enabled, egress_type, egress_tunnel_id, created_at, updated_at
	FROM peers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.Peer
	for rows.Next() {
		p, err := scanPeer(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func scanPeer(sc interface {
	Scan(dest ...any) error
}) (domain.Peer, error) {
	var p domain.Peer
	var psk sql.NullString
	var egress sql.NullInt64
	var created, updated string
	err := sc.Scan(&p.ID, &p.Name, &p.PublicKey, &p.PrivateKey, &psk, &p.AllowedIPs, &p.Enabled, &p.EgressType, &egress, &created, &updated)
	if err != nil {
		return p, err
	}
	if psk.Valid {
		s := psk.String
		p.PresharedKey = &s
	}
	if egress.Valid {
		v := egress.Int64
		p.EgressTunnelID = &v
	}
	var err2 error
	p.CreatedAt, err2 = time.Parse(time.RFC3339, created)
	if err2 != nil {
		p.CreatedAt = time.Time{}
	}
	p.UpdatedAt, err2 = time.Parse(time.RFC3339, updated)
	if err2 != nil {
		p.UpdatedAt = time.Time{}
	}
	return p, nil
}

func (s *Store) GetPeer(ctx context.Context, id int64) (domain.Peer, error) {
	row := s.db.QueryRowContext(ctx, `
	SELECT id, name, public_key, private_key, preshared_key, allowed_ips, enabled, egress_type, egress_tunnel_id, created_at, updated_at
	FROM peers WHERE id = ?`, id)
	return scanPeer(row)
}

func (s *Store) InsertPeer(ctx context.Context, p domain.Peer) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var psk any
	if p.PresharedKey != nil {
		psk = *p.PresharedKey
	}
	var egress any
	if p.EgressTunnelID != nil {
		egress = *p.EgressTunnelID
	}
	res, err := s.db.ExecContext(ctx, `
	INSERT INTO peers (name, private_key, public_key, preshared_key, allowed_ips, enabled, egress_type, egress_tunnel_id, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.PrivateKey, p.PublicKey, psk, p.AllowedIPs, p.Enabled, string(p.EgressType), egress, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdatePeer(ctx context.Context, p domain.Peer) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var psk any
	if p.PresharedKey != nil {
		psk = *p.PresharedKey
	}
	var egress any
	if p.EgressTunnelID != nil {
		egress = *p.EgressTunnelID
	}
	_, err := s.db.ExecContext(ctx, `
	UPDATE peers SET name = ?, private_key = ?, public_key = ?, preshared_key = ?, allowed_ips = ?, enabled = ?, egress_type = ?, egress_tunnel_id = ?, updated_at = ?
	WHERE id = ?`,
		p.Name, p.PrivateKey, p.PublicKey, psk, p.AllowedIPs, p.Enabled, string(p.EgressType), egress, now, p.ID,
	)
	return err
}

func (s *Store) DeletePeer(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM peers WHERE id = ?`, id)
	return err
}
