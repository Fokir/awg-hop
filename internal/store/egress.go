package store

import (
	"context"
	"errors"
	"time"

	"awghop/internal/domain"
)

func (s *Store) ListEgressTunnels(ctx context.Context) ([]domain.EgressTunnel, error) {
	rows, err := s.db.QueryContext(ctx, `
	SELECT id, name, interface_name, config_text, enabled, created_at, updated_at
	FROM egress_tunnels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []domain.EgressTunnel
	for rows.Next() {
		t, err := scanEgress(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func scanEgress(sc interface{ Scan(dest ...any) error }) (domain.EgressTunnel, error) {
	var t domain.EgressTunnel
	var en int
	var created, updated string
	err := sc.Scan(&t.ID, &t.Name, &t.InterfaceName, &t.ConfigText, &en, &created, &updated)
	if err != nil {
		return t, err
	}
	t.Enabled = en != 0
	var e1, e2 error
	t.CreatedAt, e1 = time.Parse(time.RFC3339, created)
	t.UpdatedAt, e2 = time.Parse(time.RFC3339, updated)
	if e1 != nil {
		t.CreatedAt = time.Time{}
	}
	if e2 != nil {
		t.UpdatedAt = time.Time{}
	}
	return t, nil
}

func (s *Store) GetEgressTunnel(ctx context.Context, id int64) (domain.EgressTunnel, error) {
	row := s.db.QueryRowContext(ctx, `
	SELECT id, name, interface_name, config_text, enabled, created_at, updated_at
	FROM egress_tunnels WHERE id = ?`, id)
	t, err := scanEgress(row)
	if err != nil {
		return t, err
	}
	return t, nil
}

func (s *Store) InsertEgressTunnel(ctx context.Context, t domain.EgressTunnel) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var en int
	if t.Enabled {
		en = 1
	}
	res, err := s.db.ExecContext(ctx, `
	INSERT INTO egress_tunnels (name, interface_name, config_text, enabled, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)`,
		t.Name, t.InterfaceName, t.ConfigText, en, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateEgressTunnel(ctx context.Context, t domain.EgressTunnel) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var en int
	if t.Enabled {
		en = 1
	}
	_, err := s.db.ExecContext(ctx, `
	UPDATE egress_tunnels SET name = ?, interface_name = ?, config_text = ?, enabled = ?, updated_at = ?
	WHERE id = ?`,
		t.Name, t.InterfaceName, t.ConfigText, en, now, t.ID,
	)
	return err
}

func (s *Store) DeleteEgressTunnel(ctx context.Context, id int64) error {
	n, err := s.CountPeersOnEgressTunnel(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrEgressInUse
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM egress_tunnels WHERE id = ?`, id)
	return err
}

var ErrEgressInUse = errors.New("egress tunnel is referenced by peers")

func (s *Store) CountPeersOnEgressTunnel(ctx context.Context, tunnelID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM peers WHERE egress_tunnel_id = ?`, tunnelID).Scan(&n)
	return n, err
}
