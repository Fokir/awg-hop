package domain

import "time"

// TunnelOfflinePolicy управляет поведением Apply при недоступности (disabled/down) исходящего туннеля.
type TunnelOfflinePolicy string

const (
	// TunnelOfflineBlock — отказать в Apply, если у пира выбран недоступный туннель.
	TunnelOfflineBlock TunnelOfflinePolicy = "block"
	// TunnelOfflineIgnore — игнорировать пира (правило не создавать), Apply продолжается.
	TunnelOfflineIgnore TunnelOfflinePolicy = "ignore"
)

// SystemSettings — глобальные параметры узла, не относящиеся к одному ingress-серверу.
type SystemSettings struct {
	ExternalInterface   string              `json:"external_interface"`
	TunnelOfflinePolicy TunnelOfflinePolicy `json:"tunnel_offline_policy"`
	ClientAllowedIPv4   string              `json:"client_allowed_ipv4"`
	ClientAllowedIPv6   string              `json:"client_allowed_ipv6"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

// PeerStatus — runtime-метрики пира из `awg show <iface> dump`.
type PeerStatus struct {
	PublicKey         string `json:"public_key"`
	Endpoint          string `json:"endpoint,omitempty"`
	LatestHandshake   int64  `json:"latest_handshake_unix"`
	TransferRxBytes   int64  `json:"transfer_rx_bytes"`
	TransferTxBytes   int64  `json:"transfer_tx_bytes"`
	PersistentKeepAlv int    `json:"persistent_keepalive,omitempty"`
}

// EgressTunnelStatus — runtime-данные исходящего туннеля.
type EgressTunnelStatus struct {
	InterfaceUp     bool   `json:"interface_up"`
	LatestHandshake int64  `json:"latest_handshake_unix"`
	TransferRxBytes int64  `json:"transfer_rx_bytes"`
	TransferTxBytes int64  `json:"transfer_tx_bytes"`
	LastError       string `json:"last_error,omitempty"`
}
