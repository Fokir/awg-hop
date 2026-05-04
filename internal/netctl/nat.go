package netctl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const natStateFile = "nat-state.json"

// NATRule описывает применённое правило MASQUERADE.
type NATRule struct {
	// Spec — точные аргументы (без -A/-D), которые однозначно идентифицируют правило.
	// Пример: ["-s","10.8.0.5/32","-o","eth0","-j","MASQUERADE"].
	Spec []string `json:"spec"`
}

type natState struct {
	Version int       `json:"version"`
	Rules   []NATRule `json:"rules"`
}

func (c *Controller) natStatePath() string {
	return filepath.Join(c.DataDir, natStateFile)
}

func (c *Controller) loadNATState() (*natState, error) {
	b, err := os.ReadFile(c.natStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &natState{Version: 1}, nil
		}
		return nil, err
	}
	var s natState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Controller) saveNATState(s *natState) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.natStatePath(), b, 0o600)
}

// undoNAT удаляет ранее применённые правила; ошибки логируются, но не прерывают цепочку.
func (c *Controller) undoNAT(ctx context.Context, prev *natState) {
	if runtime.GOOS != "linux" || prev == nil {
		return
	}
	for _, r := range prev.Rules {
		args := append([]string{"-t", "nat", "-D", "POSTROUTING"}, r.Spec...)
		if _, err := c.Runner.Run(ctx, c.IPTablesBin, args...); err != nil {
			c.Log.Warn("nat: failed to remove rule", "spec", r.Spec, "err", err)
		}
	}
}

// applyNAT добавляет MASQUERADE-правила.
func (c *Controller) applyNAT(ctx context.Context, rules []NATRule) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	for _, r := range rules {
		args := append([]string{"-t", "nat", "-A", "POSTROUTING"}, r.Spec...)
		if _, err := c.Runner.Run(ctx, c.IPTablesBin, args...); err != nil {
			return fmt.Errorf("iptables -A POSTROUTING %v: %w", r.Spec, err)
		}
	}
	return nil
}

// detectExternalInterface через `ip route get 1.1.1.1` и/или конфиг.
// Если override задан — используем его без проверки.
func (c *Controller) detectExternalInterface(ctx context.Context, override string) (string, error) {
	if v := strings.TrimSpace(override); v != "" {
		return v, nil
	}
	if runtime.GOOS != "linux" {
		return "", nil
	}
	out, err := c.Runner.Run(ctx, c.IPBin, "route", "get", "1.1.1.1")
	if err != nil {
		return "", fmt.Errorf("ip route get: %w", err)
	}
	// Пример: "1.1.1.1 via 172.17.0.1 dev eth0 src 172.17.0.5 uid 0"
	fields := strings.Fields(out)
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", fmt.Errorf("cannot parse external interface from: %q", strings.TrimSpace(out))
}

// makeMasqueradeRule собирает spec для одного пира.
func makeMasqueradeRule(peerCIDR, outIface string) NATRule {
	return NATRule{Spec: []string{"-s", peerCIDR, "-o", outIface, "-j", "MASQUERADE"}}
}
