package store

import (
	"context"
	"errors"
	"time"

	"awghop/internal/domain"
)

func (s *Store) ListUpstreamTunnels(ctx context.Context) ([]domain.UpstreamTunnel, error) {
	rows, err := s.db.QueryContext(ctx, `
	SELECT id, name, interface_name, config_text, enabled, created_at, updated_at
	FROM upstream_tunnels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.UpstreamTunnel
	for rows.Next() {
		t, err := scanUpstream(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func scanUpstream(sc interface{ Scan(dest ...any) error }) (domain.UpstreamTunnel, error) {
	var t domain.UpstreamTunnel
	var en int
	var created, updated string
	err := sc.Scan(&t.ID, &t.Name, &t.InterfaceName, &t.ConfigText, &en, &created, &updated)
	if err != nil {
		return t, err
	}
	t.Enabled = en != 0
	if pt, err := time.Parse(time.RFC3339, created); err == nil {
		t.CreatedAt = pt
	}
	if pt, err := time.Parse(time.RFC3339, updated); err == nil {
		t.UpdatedAt = pt
	}
	return t, nil
}

func (s *Store) GetUpstreamTunnel(ctx context.Context, id int64) (domain.UpstreamTunnel, error) {
	row := s.db.QueryRowContext(ctx, `
	SELECT id, name, interface_name, config_text, enabled, created_at, updated_at
	FROM upstream_tunnels WHERE id = ?`, id)
	return scanUpstream(row)
}

func (s *Store) InsertUpstreamTunnel(ctx context.Context, t domain.UpstreamTunnel) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var en int
	if t.Enabled {
		en = 1
	}
	res, err := s.db.ExecContext(ctx, `
	INSERT INTO upstream_tunnels (name, interface_name, config_text, enabled, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)`,
		t.Name, t.InterfaceName, t.ConfigText, en, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateUpstreamTunnel(ctx context.Context, t domain.UpstreamTunnel) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var en int
	if t.Enabled {
		en = 1
	}
	_, err := s.db.ExecContext(ctx, `
	UPDATE upstream_tunnels SET name = ?, interface_name = ?, config_text = ?, enabled = ?, updated_at = ?
	WHERE id = ?`,
		t.Name, t.InterfaceName, t.ConfigText, en, now, t.ID,
	)
	return err
}

func (s *Store) DeleteUpstreamTunnel(ctx context.Context, id int64) error {
	n, err := s.CountClientsOnUpstreamTunnel(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrUpstreamInUse
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM upstream_tunnels WHERE id = ?`, id)
	return err
}

// ErrUpstreamInUse возвращается, когда upstream-туннель ссылается хотя бы один client.
var ErrUpstreamInUse = errors.New("upstream tunnel is referenced by clients")

func (s *Store) CountClientsOnUpstreamTunnel(ctx context.Context, tunnelID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM clients WHERE upstream_tunnel_id = ?`, tunnelID).Scan(&n)
	return n, err
}
