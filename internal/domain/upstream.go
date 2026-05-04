package domain

import "time"

// UpstreamTunnel — наше исходящее AmneziaWG-подключение к удалённому AWG-серверу.
// Внутри контейнера это отдельный сетевой интерфейс (например awg1), который
// поднимается через `awg-quick up <config>`. Используется как «выход в интернет
// через удалённый узел» для выбранных клиентов.
type UpstreamTunnel struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	InterfaceName string    `json:"interface_name"`
	ConfigText    string    `json:"config_text"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// RoutingTableID — детерминированный номер policy-routing таблицы для upstream'а
// (Linux rt_tables).
func RoutingTableID(upstreamDBID int64) int {
	return int(10000 + upstreamDBID)
}

// RulePreference — уникальный pref для `ip rule` по client.id.
func RulePreference(clientID int64) int {
	return int(32000 + clientID)
}
