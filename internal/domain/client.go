package domain

import "time"

// UpstreamType — куда направляется трафик клиента после декапсуляции на нашем
// входном AmneziaWG-сервере.
type UpstreamType string

const (
	// UpstreamDirect — выход в интернет напрямую через NAT внешнего интерфейса контейнера.
	UpstreamDirect UpstreamType = "direct"
	// UpstreamViaTunnel — трафик клиента уходит в одно из наших исходящих
	// AmneziaWG-подключений (наш сервер выступает клиентом другого AWG-сервера).
	UpstreamViaTunnel UpstreamType = "via_upstream"
)

// Client — устройство пользователя, подключающееся к нашему входному AWG-серверу.
//
// Это аналог "client" в wg-easy. Привязан к одному IP-адресу в туннельной подсети
// и опционально к выбранному upstream-туннелю (UpstreamType=via_upstream).
type Client struct {
	ID                int64        `json:"id"`
	Name              string       `json:"name"`
	PublicKey         string       `json:"public_key"`
	PrivateKey        string       `json:"-"`
	PresharedKey      *string      `json:"preshared_key,omitempty"`
	AllowedIPs        string       `json:"allowed_ips"`
	Enabled           bool         `json:"enabled"`
	UpstreamType      UpstreamType `json:"upstream_type"`
	UpstreamTunnelID  *int64       `json:"upstream_tunnel_id,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

// IngressSettings — настройки нашего собственного AmneziaWG-сервера, к которому
// подключаются клиентские устройства.
type IngressSettings struct {
	ListenPort       int    `json:"listen_port"`
	HostEndpoint     string `json:"host_endpoint"`
	TunnelSubnet     string `json:"tunnel_subnet"`
	DNSServers       string `json:"dns_servers"`
	MTU              int    `json:"mtu"`
	InterfaceName    string `json:"interface_name"`
	ServerTunnelIP   string `json:"server_tunnel_ip"`
	ServerPublicKey  string `json:"server_public_key"`
	ServerPrivateKey string `json:"-"`
	Jc               int    `json:"jc"`
	Jmin             int    `json:"jmin"`
	Jmax             int    `json:"jmax"`
	S1               string `json:"s1"`
	S2               string `json:"s2"`
	S3               string `json:"s3"`
	S4               string `json:"s4"`
	H1               int64  `json:"h1"`
	H2               int64  `json:"h2"`
	H3               int64  `json:"h3"`
	H4               int64  `json:"h4"`
}

type SetupStatus struct {
	SetupComplete bool `json:"setup_complete"`
}
