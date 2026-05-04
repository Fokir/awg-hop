package awgshow

import "testing"

func TestParseEmpty(t *testing.T) {
	got, err := Parse("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || len(got.Peers) != 0 {
		t.Fatalf("expected empty interface, got %+v", got)
	}
}

func TestParseInterfaceOnly(t *testing.T) {
	dump := "AAAA\tBBBB\t51820\toff\n"
	got, err := Parse(dump)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.PublicKey != "BBBB" {
		t.Fatalf("public key: %q", got.PublicKey)
	}
	if got.ListenPort != 51820 {
		t.Fatalf("listen port: %d", got.ListenPort)
	}
	if got.FWMark != "off" {
		t.Fatalf("fwmark: %q", got.FWMark)
	}
	if len(got.Peers) != 0 {
		t.Fatalf("peers: %d", len(got.Peers))
	}
}

func TestParseWithPeers(t *testing.T) {
	dump := "PRIVKEY\tPUBKEY\t51820\toff\n" +
		"PEER1PUB\t(none)\t1.2.3.4:51820\t10.8.0.5/32\t1700000000\t1024\t2048\t25\n" +
		"PEER2PUB\tPSK==\t(none)\t10.8.0.6/32, 10.8.0.7/32\t0\t0\t0\toff\n"
	got, err := Parse(dump)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Peers) != 2 {
		t.Fatalf("want 2 peers, got %d", len(got.Peers))
	}
	p1 := got.Peers[0]
	if p1.PublicKey != "PEER1PUB" || p1.PresharedKey != "" || p1.Endpoint != "1.2.3.4:51820" {
		t.Fatalf("peer1 mismatch: %+v", p1)
	}
	if p1.LatestHandshake != 1700000000 || p1.TransferRxBytes != 1024 || p1.TransferTxBytes != 2048 || p1.PersistentKeepAlv != 25 {
		t.Fatalf("peer1 metrics: %+v", p1)
	}
	p2 := got.Peers[1]
	if p2.PublicKey != "PEER2PUB" || p2.PresharedKey != "PSK==" || p2.Endpoint != "" {
		t.Fatalf("peer2 mismatch: %+v", p2)
	}
	if p2.PersistentKeepAlv != 0 {
		t.Fatalf("peer2 keepalive: %d", p2.PersistentKeepAlv)
	}
}
