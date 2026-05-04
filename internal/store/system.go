package store

import (
	"context"
	"time"

	"awghop/internal/domain"
)

func (s *Store) GetSystemSettings(ctx context.Context) (domain.SystemSettings, error) {
	var ss domain.SystemSettings
	var policy, updated string
	err := s.db.QueryRowContext(ctx, `
	SELECT external_interface, tunnel_offline_policy, client_allowed_ipv4, client_allowed_ipv6, updated_at
	FROM system_settings WHERE id = 1`).Scan(
		&ss.ExternalInterface, &policy, &ss.ClientAllowedIPv4, &ss.ClientAllowedIPv6, &updated,
	)
	if err != nil {
		return ss, err
	}
	ss.TunnelOfflinePolicy = domain.TunnelOfflinePolicy(policy)
	if t, err := time.Parse(time.RFC3339, updated); err == nil {
		ss.UpdatedAt = t
	}
	return ss, nil
}

func (s *Store) UpdateSystemSettings(ctx context.Context, ss domain.SystemSettings) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
	UPDATE system_settings SET
		external_interface = ?,
		tunnel_offline_policy = ?,
		client_allowed_ipv4 = ?,
		client_allowed_ipv6 = ?,
		updated_at = ?
	WHERE id = 1`,
		ss.ExternalInterface, string(ss.TunnelOfflinePolicy), ss.ClientAllowedIPv4, ss.ClientAllowedIPv6, now,
	)
	return err
}

// ReplaceAdminPassword обновляет хэш пароля. Используется бэкап-импортом и сменой пароля.
func (s *Store) ReplaceAdminPassword(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE admin_account SET password_hash = ? WHERE id = 1`, hash)
	return err
}
