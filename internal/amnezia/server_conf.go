package amnezia

import (
	"fmt"
	"net"
	"strings"

	"awghop/internal/domain"
)

// BuildIngressServerConf собирает AmneziaWG/WireGuard конфиг для входного интерфейса сервера.
//
// Каждый client становится секцией [Peer] в серверном конфиге.
func BuildIngressServerConf(s domain.IngressSettings, clients []domain.Client) string {
	addr := ingressInterfaceAddress(s.ServerTunnelIP, s.TunnelSubnet)
	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", strings.TrimSpace(s.ServerPrivateKey))
	fmt.Fprintf(&b, "Address = %s\n", addr)
	fmt.Fprintf(&b, "ListenPort = %d\n", s.ListenPort)
	fmt.Fprintf(&b, "MTU = %d\n", s.MTU)
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

	for _, c := range clients {
		if !c.Enabled {
			continue
		}
		fmt.Fprintf(&b, "\n[Peer]\n")
		fmt.Fprintf(&b, "PublicKey = %s\n", strings.TrimSpace(c.PublicKey))
		fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.TrimSpace(c.AllowedIPs))
		if c.PresharedKey != nil && strings.TrimSpace(*c.PresharedKey) != "" {
			fmt.Fprintf(&b, "PresharedKey = %s\n", *c.PresharedKey)
		}
	}
	return b.String()
}

func ingressInterfaceAddress(serverTunnelIP, tunnelSubnet string) string {
	ip := strings.TrimSpace(serverTunnelIP)
	if i := strings.Index(ip, "/"); i >= 0 {
		ip = ip[:i]
	}
	_, n, err := net.ParseCIDR(strings.TrimSpace(tunnelSubnet))
	if err != nil {
		return fmt.Sprintf("%s/24", ip)
	}
	ones, bits := n.Mask.Size()
	if bits != 32 {
		return fmt.Sprintf("%s/%d", ip, ones)
	}
	return fmt.Sprintf("%s/%d", ip, ones)
}
