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
