package wgeasy

import (
	"errors"
	"strings"
	"testing"
)

const sampleAmnezia = `{
  "server": {
    "privateKey": "PRIV==",
    "publicKey": "PUB==",
    "address": "10.8.0.1/24",
    "port": 51820,
    "endpoint": "vpn.example.com:51820",
    "mtu": 1420,
    "amnezia": {
      "jc": 5, "jmin": 50, "jmax": 1000,
      "s1": "", "s2": "", "s3": "", "s4": "",
      "h1": 1, "h2": 2, "h3": 3, "h4": 4
    }
  },
  "clients": {
    "abc": {
      "name": "alice",
      "privateKey": "CPRIV==",
      "publicKey": "CPUB==",
      "address": "10.8.0.5",
      "enabled": true
    },
    "def": {
      "name": "",
      "privateKey": "DPRIV==",
      "publicKey": "DPUB==",
      "address": "10.8.0.6/32",
      "preSharedKey": "PSK==",
      "enabled": false
    }
  }
}`

const sampleVanilla = `{
  "server": {
    "privateKey": "PRIV==",
    "publicKey": "PUB==",
    "address": "10.8.0.1/24",
    "port": 51820
  },
  "clients": {}
}`

func TestParseAmneziaExport(t *testing.T) {
	res, err := Parse(strings.NewReader(sampleAmnezia))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Ingress.Jc != 5 || res.Ingress.Jmax != 1000 {
		t.Fatalf("ingress amnezia fields not picked up: %+v", res.Ingress)
	}
	if res.Ingress.HostEndpoint != "vpn.example.com:51820" {
		t.Fatalf("endpoint: %q", res.Ingress.HostEndpoint)
	}
	if res.Ingress.TunnelSubnet != "10.8.0.0/24" {
		t.Fatalf("subnet: %q", res.Ingress.TunnelSubnet)
	}
	if len(res.Clients) != 2 {
		t.Fatalf("want 2 clients, got %d", len(res.Clients))
	}
	for _, c := range res.Clients {
		if !strings.HasSuffix(c.AllowedIPs, "/32") {
			t.Fatalf("client allowed_ips not /32: %q", c.AllowedIPs)
		}
	}
}

func TestParseVanillaRejected(t *testing.T) {
	_, err := Parse(strings.NewReader(sampleVanilla))
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("want ErrUnsupportedFormat, got %v", err)
	}
}
