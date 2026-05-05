package amnezia

import (
	"strconv"
	"testing"

	"awghop/internal/domain"
)

func TestEnsureAmneziaDefaults_FillsZeroFields(t *testing.T) {
	in := domain.IngressSettings{}
	changed, err := EnsureAmneziaDefaults(&in)
	if err != nil {
		t.Fatalf("EnsureAmneziaDefaults: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true on empty input")
	}

	if in.S1 == "" || in.S2 == "" {
		t.Fatalf("S1/S2 must be set: S1=%q S2=%q", in.S1, in.S2)
	}
	s1, err := strconv.Atoi(in.S1)
	if err != nil {
		t.Fatalf("S1 not numeric: %q", in.S1)
	}
	s2, err := strconv.Atoi(in.S2)
	if err != nil {
		t.Fatalf("S2 not numeric: %q", in.S2)
	}
	if s1 < junkPaddingMin || s1 > junkPaddingMax {
		t.Errorf("S1 out of range: %d not in [%d,%d]", s1, junkPaddingMin, junkPaddingMax)
	}
	if s2 < junkPaddingMin || s2 > junkPaddingMax {
		t.Errorf("S2 out of range: %d not in [%d,%d]", s2, junkPaddingMin, junkPaddingMax)
	}
	if s1 == s2 {
		t.Errorf("S1 and S2 must differ: %d", s1)
	}
	if s1+initiationOverhead == s2 {
		t.Errorf("S1+%d must not equal S2; got S1=%d S2=%d", initiationOverhead, s1, s2)
	}

	hs := []int64{in.H1, in.H2, in.H3, in.H4}
	seen := map[int64]struct{}{}
	for i, h := range hs {
		if h <= 0 {
			t.Errorf("H%d must be positive, got %d", i+1, h)
		}
		if h == 1 || h == 2 || h == 3 || h == 4 {
			t.Errorf("H%d must not equal WG message type, got %d", i+1, h)
		}
		if _, dup := seen[h]; dup {
			t.Errorf("H%d duplicates earlier marker: %d", i+1, h)
		}
		seen[h] = struct{}{}
	}
}

func TestEnsureAmneziaDefaults_PreservesUserValues(t *testing.T) {
	in := domain.IngressSettings{
		S1: "33", S2: "74",
		H1: 100, H2: 200, H3: 300, H4: 400,
	}
	changed, err := EnsureAmneziaDefaults(&in)
	if err != nil {
		t.Fatalf("EnsureAmneziaDefaults: %v", err)
	}
	if changed {
		t.Fatalf("expected changed=false when all fields already set")
	}
	if in.S1 != "33" || in.S2 != "74" {
		t.Errorf("S1/S2 must be preserved: S1=%q S2=%q", in.S1, in.S2)
	}
	if in.H1 != 100 || in.H2 != 200 || in.H3 != 300 || in.H4 != 400 {
		t.Errorf("H1..H4 must be preserved: %d %d %d %d", in.H1, in.H2, in.H3, in.H4)
	}
}

func TestEnsureAmneziaDefaults_FillsOnlyMissingScalars(t *testing.T) {
	in := domain.IngressSettings{
		S1: "42",
		H1: 11, H2: 22, H3: 33, H4: 44,
	}
	changed, err := EnsureAmneziaDefaults(&in)
	if err != nil {
		t.Fatalf("EnsureAmneziaDefaults: %v", err)
	}
	if !changed {
		t.Fatalf("expected changed=true: S2 was empty")
	}
	if in.S1 != "42" {
		t.Errorf("S1 must be preserved, got %q", in.S1)
	}
	if in.S2 == "" {
		t.Errorf("S2 must be filled")
	}
	if in.H1 != 11 || in.H2 != 22 || in.H3 != 33 || in.H4 != 44 {
		t.Errorf("H1..H4 must be preserved: %d %d %d %d", in.H1, in.H2, in.H3, in.H4)
	}
}
