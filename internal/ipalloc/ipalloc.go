package ipalloc

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"
)

// NextPeerPrefix picks the next available /32 inside an IPv4 subnet (usable hosts only).
func NextPeerPrefix(tunnelSubnet, serverIP string, usedAllowedIPs []string) (netip.Prefix, error) {
	prefix, err := netip.ParsePrefix(tunnelSubnet)
	if err != nil {
		return netip.Prefix{}, err
	}
	if !prefix.Addr().Is4() {
		return netip.Prefix{}, fmt.Errorf("only IPv4 subnet supported in MVP")
	}
	serverAddr, err := netip.ParseAddr(strings.Split(serverIP, "/")[0])
	if err != nil {
		return netip.Prefix{}, err
	}
	used := make(map[uint32]struct{})
	for _, s := range usedAllowedIPs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if p, err := netip.ParsePrefix(s); err == nil {
			used[addrKey(p.Addr())] = struct{}{}
			continue
		}
		if a, err := netip.ParseAddr(strings.Split(s, "/")[0]); err == nil {
			used[addrKey(a)] = struct{}{}
		}
	}

	n := prefix.Bits()
	if n < 0 || n > 32 {
		return netip.Prefix{}, fmt.Errorf("invalid prefix length")
	}
	base := binary.BigEndian.Uint32(prefix.Masked().Addr().AsSlice())
	hostBits := 32 - n
	if hostBits > 31 {
		return netip.Prefix{}, fmt.Errorf("subnet too large")
	}
	size := uint32(1) << hostBits
	first := base + 1
	last := base + size - 2
	srv := addrKey(serverAddr)

	for ip := first; ip <= last; ip++ {
		if ip == srv {
			continue
		}
		if _, ok := used[ip]; ok {
			continue
		}
		a := uint32ToAddr(ip)
		pr, err := a.Prefix(32)
		if err != nil {
			continue
		}
		return pr, nil
	}
	return netip.Prefix{}, fmt.Errorf("no free addresses in %s", tunnelSubnet)
}

func addrKey(a netip.Addr) uint32 {
	b := a.As4()
	return binary.BigEndian.Uint32(b[:])
}

func uint32ToAddr(u uint32) netip.Addr {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], u)
	return netip.AddrFrom4(b)
}
