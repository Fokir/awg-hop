// Package awgshow parses the output of `awg show <iface> dump` (compatible with `wg show`).
package awgshow

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Runner совместим с netctl.Runner, но описан локально, чтобы не плодить зависимости.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// Interface — состояние интерфейса AmneziaWG/WireGuard.
type Interface struct {
	PublicKey  string
	ListenPort int
	FWMark     string
	Peers      []Peer
}

// Peer — одна строка peer-секции дампа.
type Peer struct {
	PublicKey         string
	PresharedKey      string
	Endpoint          string
	AllowedIPs        string
	LatestHandshake   int64
	TransferRxBytes   int64
	TransferTxBytes   int64
	PersistentKeepAlv int
}

// Show вызывает `awg show <iface> dump` (или `wg show`, если AmneziaWG не установлен)
// и парсит результат.
func Show(ctx context.Context, runner Runner, awgBin, iface string) (*Interface, error) {
	if awgBin == "" {
		awgBin = "awg"
	}
	out, err := runner.Run(ctx, awgBin, "show", iface, "dump")
	if err != nil {
		return nil, err
	}
	return Parse(out)
}

// Parse разбирает текстовый дамп.
//
// Первая строка — секция интерфейса: <priv|none>\t<pub>\t<port>\t<fwmark>.
// Каждая следующая строка — peer: <pub>\t<psk|none>\t<endpoint|none>\t<allowed>\t<handshake>\t<rx>\t<tx>\t<keepalive|off>.
func Parse(dump string) (*Interface, error) {
	lines := strings.Split(strings.TrimRight(dump, "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return &Interface{}, nil
	}

	var iface Interface

	headFields := strings.Split(lines[0], "\t")
	if len(headFields) >= 4 {
		iface.PublicKey = strings.TrimSpace(headFields[1])
		if p, err := strconv.Atoi(strings.TrimSpace(headFields[2])); err == nil {
			iface.ListenPort = p
		}
		iface.FWMark = strings.TrimSpace(headFields[3])
	}

	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 8 {
			return nil, fmt.Errorf("awgshow: peer line has %d fields, want 8: %q", len(f), line)
		}
		var p Peer
		p.PublicKey = strings.TrimSpace(f[0])
		if v := strings.TrimSpace(f[1]); v != "" && v != "(none)" {
			p.PresharedKey = v
		}
		if v := strings.TrimSpace(f[2]); v != "" && v != "(none)" {
			p.Endpoint = v
		}
		p.AllowedIPs = strings.TrimSpace(f[3])
		p.LatestHandshake, _ = strconv.ParseInt(strings.TrimSpace(f[4]), 10, 64)
		p.TransferRxBytes, _ = strconv.ParseInt(strings.TrimSpace(f[5]), 10, 64)
		p.TransferTxBytes, _ = strconv.ParseInt(strings.TrimSpace(f[6]), 10, 64)
		if v := strings.TrimSpace(f[7]); v != "" && v != "off" {
			p.PersistentKeepAlv, _ = strconv.Atoi(v)
		}
		iface.Peers = append(iface.Peers, p)
	}
	return &iface, nil
}
