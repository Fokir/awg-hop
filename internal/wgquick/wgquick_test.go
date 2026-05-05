package wgquick

import (
	"strings"
	"testing"
)

func TestStripInterfaceDirective_RemovesDNSFromInterfaceOnly(t *testing.T) {
	in := strings.Join([]string{
		"[Interface]",
		"Address = 10.8.1.22/32",
		"DNS = 172.29.172.254, 208.67.220.220",
		"PrivateKey = abc",
		"",
		"[Peer]",
		"PublicKey = xyz",
		"Endpoint = 1.2.3.4:5",
		"AllowedIPs = 0.0.0.0/0",
		"",
	}, "\n")

	got := StripInterfaceDirective(in, "DNS")

	if strings.Contains(got, "DNS =") {
		t.Errorf("DNS line must be removed, got:\n%s", got)
	}
	for _, must := range []string{
		"[Interface]", "Address = 10.8.1.22/32", "PrivateKey = abc",
		"[Peer]", "PublicKey = xyz", "Endpoint = 1.2.3.4:5", "AllowedIPs = 0.0.0.0/0",
	} {
		if !strings.Contains(got, must) {
			t.Errorf("expected to keep %q, got:\n%s", must, got)
		}
	}
}

func TestStripInterfaceDirective_PreservesCRLFAndComments(t *testing.T) {
	in := "[Interface]\r\nAddress = 10.0.0.1/24\r\n# Comment about DNS\r\nDNS = 8.8.8.8\r\nPrivateKey = key\r\n"
	got := StripInterfaceDirective(in, "DNS")

	if !strings.Contains(got, "\r\n") {
		t.Errorf("CRLF must be preserved, got: %q", got)
	}
	if !strings.Contains(got, "# Comment about DNS") {
		t.Errorf("comment must be preserved, got: %q", got)
	}
	if strings.Contains(got, "DNS = 8.8.8.8") {
		t.Errorf("DNS must be removed, got: %q", got)
	}
}

func TestStripInterfaceDirective_DoesNotRemoveOutsideInterface(t *testing.T) {
	in := "[Other]\nDNS = 1.1.1.1\n[Interface]\nAddress = 10.0.0.1/24\n"
	got := StripInterfaceDirective(in, "DNS")
	if !strings.Contains(got, "DNS = 1.1.1.1") {
		t.Errorf("DNS in [Other] section must be kept, got:\n%s", got)
	}
}

func TestStripInterfaceDirective_CaseInsensitiveKey(t *testing.T) {
	in := "[Interface]\ndns = 1.1.1.1\nDNS = 2.2.2.2\nDns = 3.3.3.3\n"
	got := StripInterfaceDirective(in, "DNS")
	if strings.Contains(strings.ToLower(got), "dns =") {
		t.Errorf("all DNS variants must be removed, got:\n%s", got)
	}
}

func TestStripInterfaceDirective_NoOpWithoutKeys(t *testing.T) {
	in := "[Interface]\nDNS = 1.1.1.1\n"
	got := StripInterfaceDirective(in)
	if got != in {
		t.Errorf("no keys -> input unchanged, got:\n%s", got)
	}
}

func TestSetInterfaceDirective_AddsKeyAfterInterfaceHeader(t *testing.T) {
	in := "[Interface]\nAddress = 10.0.0.1/24\nPrivateKey = abc\n\n[Peer]\nPublicKey = xyz\n"
	got := SetInterfaceDirective(in, "Table", "off")
	if !strings.Contains(got, "[Interface]\nTable = off\n") {
		t.Errorf("Table = off must be inserted right after [Interface], got:\n%s", got)
	}
	if !strings.Contains(got, "Address = 10.0.0.1/24") || !strings.Contains(got, "PublicKey = xyz") {
		t.Errorf("other content must be preserved, got:\n%s", got)
	}
	if strings.Count(got, "Table = off") != 1 {
		t.Errorf("Table = off must appear exactly once, got:\n%s", got)
	}
}

func TestSetInterfaceDirective_ReplacesExistingValue(t *testing.T) {
	in := "[Interface]\nAddress = 10.0.0.1/24\nTable = 51820\nPrivateKey = abc\n"
	got := SetInterfaceDirective(in, "Table", "off")
	if strings.Contains(got, "Table = 51820") {
		t.Errorf("old Table value must be removed, got:\n%s", got)
	}
	if strings.Count(got, "Table = off") != 1 {
		t.Errorf("Table = off must appear exactly once, got:\n%s", got)
	}
}

func TestSetInterfaceDirective_OnlyTouchesInterfaceSection(t *testing.T) {
	in := "[Peer]\nTable = 5\n[Interface]\nPrivateKey = abc\n"
	got := SetInterfaceDirective(in, "Table", "off")
	if !strings.Contains(got, "[Peer]\nTable = 5") {
		t.Errorf("Table in [Peer] must be untouched, got:\n%s", got)
	}
}
