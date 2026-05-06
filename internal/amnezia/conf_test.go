package amnezia

import (
	"strings"
	"testing"

	"awghop/internal/domain"
)

func TestBuildClientConf_NoMTU_IncludesS3S4(t *testing.T) {
	c := domain.Client{
		PrivateKey: "YKdLJ6P+Tv5K8lJ8qNJXyqNJXyqNJXyqNJXyqNJXyqNJU=",
		AllowedIPs: "10.99.20.2/32",
	}
	in := domain.IngressSettings{
		HostEndpoint:    "example.com:51821",
		DNSServers:      "1.1.1.1",
		MTU:             1420,
		ServerPublicKey: "abc+defGhijkLmNoPqrStUvWxYz012345678901234567890=",
		Jc:              4, Jmin: 50, Jmax: 1000,
		S1: "30", S2: "110", S3: "25", S4: "40",
		H1: 10, H2: 20, H3: 30, H4: 40,
	}
	sys := domain.SystemSettings{ClientAllowedIPv4: "0.0.0.0/0"}

	out := BuildClientConf(c, in, sys)
	if strings.Contains(out, "MTU =") {
		t.Fatalf("client conf must not contain MTU line; got:\n%s", out)
	}
	if !strings.Contains(out, "S3 = 25") || !strings.Contains(out, "S4 = 40") {
		t.Fatalf("expected S3/S4 in client conf; got:\n%s", out)
	}
}
