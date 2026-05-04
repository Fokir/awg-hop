// Package wgeasy реализует разбор экспорта wg-easy с включённым AmneziaWG.
//
// Поддерживается JSON-формат wg-easy v15+, где в файле `wg0.json` присутствуют
// поля сервера/клиентов и AmneziaWG-параметры (jc/jmin/jmax/s1..s4/h1..h4).
//
// Если файл не содержит AmneziaWG-блок (vanilla wg-easy), импорт отклоняется
// с понятной ошибкой согласно спецификации §5.5.
package wgeasy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"time"

	"awghop/internal/domain"
)

// ErrUnsupportedFormat возвращается, когда экспорт явно не AmneziaWG.
var ErrUnsupportedFormat = errors.New("wg-easy export is not AmneziaWG-compatible (vanilla WireGuard)")

// File — упрощённое представление wg-easy/wg0.json.
//
// Гибкая схема: разные сборки wg-easy кладут AmneziaWG-параметры либо на верхнем
// уровне, либо в `server.amnezia` / `awg`. Мы разбираем верхний уровень + два
// возможных вложенных объекта.
type File struct {
	Server  serverNode `json:"server"`
	Clients map[string]clientNode `json:"clients"`

	// Amnezia-параметры могут лежать рядом со server.
	AWG     *amnezia `json:"awg,omitempty"`
	Amnezia *amnezia `json:"amnezia,omitempty"`
}

type serverNode struct {
	PrivateKey string  `json:"privateKey"`
	PublicKey  string  `json:"publicKey"`
	Address    string  `json:"address"`
	Port       int     `json:"port"`
	MTU        int     `json:"mtu"`
	Endpoint   string  `json:"endpoint"`
	Subnet     string  `json:"subnet"`
	Amnezia    *amnezia `json:"amnezia,omitempty"`
	AWG        *amnezia `json:"awg,omitempty"`
}

type amnezia struct {
	JC   *int    `json:"jc"`
	Jmin *int    `json:"jmin"`
	Jmax *int    `json:"jmax"`
	S1   *string `json:"s1"`
	S2   *string `json:"s2"`
	S3   *string `json:"s3"`
	S4   *string `json:"s4"`
	H1   *int64  `json:"h1"`
	H2   *int64  `json:"h2"`
	H3   *int64  `json:"h3"`
	H4   *int64  `json:"h4"`
}

type clientNode struct {
	Name         string    `json:"name"`
	PrivateKey   string    `json:"privateKey"`
	PublicKey    string    `json:"publicKey"`
	PreSharedKey string    `json:"preSharedKey"`
	Address      string    `json:"address"`
	Enabled      *bool     `json:"enabled"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Result — результат разбора, готовый к импорту.
type Result struct {
	Ingress domain.IngressSettings
	Peers   []domain.Peer
}

// Parse читает JSON и валидирует формат.
//
// Возвращает ErrUnsupportedFormat если AmneziaWG-блок отсутствует.
func Parse(r io.Reader) (*Result, error) {
	var f File
	if err := json.NewDecoder(r).Decode(&f); err != nil {
		return nil, fmt.Errorf("parse wg-easy json: %w", err)
	}

	awg := pickAmnezia(f)
	if awg == nil {
		return nil, ErrUnsupportedFormat
	}

	if strings.TrimSpace(f.Server.PrivateKey) == "" || strings.TrimSpace(f.Server.PublicKey) == "" {
		return nil, errors.New("wg-easy: missing server keys")
	}

	in := domain.IngressSettings{
		ServerPrivateKey: strings.TrimSpace(f.Server.PrivateKey),
		ServerPublicKey:  strings.TrimSpace(f.Server.PublicKey),
		ListenPort:       defaultPort(f.Server.Port),
		HostEndpoint:     strings.TrimSpace(f.Server.Endpoint),
		MTU:              defaultInt(f.Server.MTU, 1420),
		InterfaceName:    "awg0",
		ServerTunnelIP:   strings.TrimSpace(splitFirst(f.Server.Address)),
		TunnelSubnet:     normaliseSubnet(f.Server.Address, f.Server.Subnet),
		DNSServers:       "1.1.1.1",
	}
	if awg.JC != nil {
		in.Jc = *awg.JC
	}
	if awg.Jmin != nil {
		in.Jmin = *awg.Jmin
	}
	if awg.Jmax != nil {
		in.Jmax = *awg.Jmax
	}
	if awg.S1 != nil {
		in.S1 = *awg.S1
	}
	if awg.S2 != nil {
		in.S2 = *awg.S2
	}
	if awg.S3 != nil {
		in.S3 = *awg.S3
	}
	if awg.S4 != nil {
		in.S4 = *awg.S4
	}
	if awg.H1 != nil {
		in.H1 = *awg.H1
	}
	if awg.H2 != nil {
		in.H2 = *awg.H2
	}
	if awg.H3 != nil {
		in.H3 = *awg.H3
	}
	if awg.H4 != nil {
		in.H4 = *awg.H4
	}

	res := &Result{Ingress: in}

	for id, c := range f.Clients {
		name := strings.TrimSpace(c.Name)
		if name == "" {
			name = "wg-easy-" + id
		}
		addr := strings.TrimSpace(c.Address)
		if addr == "" {
			continue
		}
		if !strings.Contains(addr, "/") {
			addr += "/32"
		}
		if _, err := netip.ParsePrefix(addr); err != nil {
			continue
		}
		enabled := true
		if c.Enabled != nil {
			enabled = *c.Enabled
		}
		var psk *string
		if v := strings.TrimSpace(c.PreSharedKey); v != "" {
			psk = &v
		}
		p := domain.Peer{
			Name:         name,
			PrivateKey:   strings.TrimSpace(c.PrivateKey),
			PublicKey:    strings.TrimSpace(c.PublicKey),
			PresharedKey: psk,
			AllowedIPs:   addr,
			Enabled:      enabled,
			EgressType:   domain.EgressDirect,
		}
		res.Peers = append(res.Peers, p)
	}

	return res, nil
}

func pickAmnezia(f File) *amnezia {
	if f.AWG != nil {
		return f.AWG
	}
	if f.Amnezia != nil {
		return f.Amnezia
	}
	if f.Server.AWG != nil {
		return f.Server.AWG
	}
	if f.Server.Amnezia != nil {
		return f.Server.Amnezia
	}
	return nil
}

func defaultPort(p int) int {
	if p <= 0 || p > 65535 {
		return 51820
	}
	return p
}

func defaultInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func splitFirst(s string) string {
	if i := strings.Index(s, "/"); i >= 0 {
		return s[:i]
	}
	return s
}

func normaliseSubnet(serverAddress, subnet string) string {
	if v := strings.TrimSpace(subnet); v != "" {
		if _, err := netip.ParsePrefix(v); err == nil {
			return v
		}
	}
	addr := strings.TrimSpace(serverAddress)
	if addr == "" {
		return "10.8.0.0/24"
	}
	if !strings.Contains(addr, "/") {
		addr += "/24"
	}
	if p, err := netip.ParsePrefix(addr); err == nil {
		return p.Masked().String()
	}
	return "10.8.0.0/24"
}
