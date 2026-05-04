package domain

import "time"

// EgressTunnel — исходящий туннель к удалённому AWG/WG-серверу (конфиг для подъёма интерфейса на хосте).
type EgressTunnel struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	InterfaceName string    `json:"interface_name"`
	ConfigText    string    `json:"config_text"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RoutingTableID детерминированный номер policy-routing таблицы для туннеля (Linux rt_tables).
func RoutingTableID(tunnelDBID int64) int {
	return int(10000 + tunnelDBID)
}

// RulePreference для ip rule (уникально по пиру).
func RulePreference(peerID int64) int {
	return int(32000 + peerID)
}
