package domain

import "time"

type EgressType string

const (
	EgressDirect    EgressType = "direct"
	EgressViaTunnel EgressType = "egress_awg"
)

type Peer struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	PublicKey      string     `json:"public_key"`
	PrivateKey     string     `json:"-"`
	PresharedKey   *string    `json:"preshared_key,omitempty"`
	AllowedIPs     string     `json:"allowed_ips"`
	Enabled        bool       `json:"enabled"`
	EgressType     EgressType `json:"egress_type"`
	EgressTunnelID *int64     `json:"egress_tunnel_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

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
