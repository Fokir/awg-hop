package amnezia

import (
	"fmt"
	"strings"

	"awghop/internal/domain"
)

// BuildClientConf рендерит AmneziaWG-конфиг для клиентского устройства.
//
// allowedIPs формируется из system-настроек (по умолчанию IPv4-only `0.0.0.0/0`).
// ServerPublicKey берётся из IngressSettings.
func BuildClientConf(c domain.Client, s domain.IngressSettings, sys domain.SystemSettings) string {
	endpoint := strings.TrimSpace(s.HostEndpoint)
	if endpoint == "" {
		endpoint = fmt.Sprintf("127.0.0.1:%d", s.ListenPort)
	}
	allowedV4 := strings.TrimSpace(sys.ClientAllowedIPv4)
	if allowedV4 == "" {
		allowedV4 = "0.0.0.0/0"
	}
	allowedV6 := strings.TrimSpace(sys.ClientAllowedIPv6)

	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", c.PrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", c.AllowedIPs)
	if strings.TrimSpace(s.DNSServers) != "" {
		fmt.Fprintf(&b, "DNS = %s\n", s.DNSServers)
	}
	// MTU не пишем в клиентский конфиг: многие «эталонные» конфиги Amnezia
	// обходятся без строки MTU — клиент/ОС подбирают разумное значение.
	// На сервере MTU по-прежнему задаётся в BuildIngressServerConf.
	fmt.Fprintf(&b, "Jc = %d\n", s.Jc)
	fmt.Fprintf(&b, "Jmin = %d\n", s.Jmin)
	fmt.Fprintf(&b, "Jmax = %d\n", s.Jmax)
	if s.S1 != "" {
		fmt.Fprintf(&b, "S1 = %s\n", s.S1)
	}
	if s.S2 != "" {
		fmt.Fprintf(&b, "S2 = %s\n", s.S2)
	}
	if s.S3 != "" {
		fmt.Fprintf(&b, "S3 = %s\n", s.S3)
	}
	if s.S4 != "" {
		fmt.Fprintf(&b, "S4 = %s\n", s.S4)
	}
	fmt.Fprintf(&b, "H1 = %d\n", s.H1)
	fmt.Fprintf(&b, "H2 = %d\n", s.H2)
	fmt.Fprintf(&b, "H3 = %d\n", s.H3)
	fmt.Fprintf(&b, "H4 = %d\n", s.H4)

	fmt.Fprintf(&b, "\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", strings.TrimSpace(s.ServerPublicKey))
	fmt.Fprintf(&b, "Endpoint = %s\n", endpoint)
	if allowedV6 != "" {
		fmt.Fprintf(&b, "AllowedIPs = %s, %s\n", allowedV4, allowedV6)
	} else {
		fmt.Fprintf(&b, "AllowedIPs = %s\n", allowedV4)
	}
	fmt.Fprintf(&b, "PersistentKeepalive = 25\n")
	if c.PresharedKey != nil && *c.PresharedKey != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", *c.PresharedKey)
	}
	return b.String()
}
