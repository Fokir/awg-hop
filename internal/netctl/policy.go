package netctl

import (
	"net/netip"
	"strings"
)

// PeerRuleSource нормализует AllowedIPs клиента до IPv4 /32 для `ip rule from`.
func PeerRuleSource(allowedIPs string) (cidr string, ok bool) {
	s := strings.TrimSpace(allowedIPs)
	if s == "" {
		return "", false
	}
	p, err := netip.ParsePrefix(s)
	if err != nil {
		a, err2 := netip.ParseAddr(strings.Split(s, "/")[0])
		if err2 != nil || !a.Is4() {
			return "", false
		}
		p2, err3 := a.Prefix(32)
		if err3 != nil {
			return "", false
		}
		return p2.String(), true
	}
	if !p.Addr().Is4() {
		return "", false
	}
	if p.Bits() != 32 {
		a := p.Addr()
		p2, err := a.Prefix(32)
		if err != nil {
			return "", false
		}
		return p2.String(), true
	}
	return p.String(), true
}
