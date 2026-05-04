package netctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	"awghop/internal/awgshow"
	"awghop/internal/config"
	"awghop/internal/domain"
	"awghop/internal/store"
)

// Controller отвечает за syncing состояния AmneziaWG/policy-routing/NAT с моделью БД.
type Controller struct {
	DataDir       string
	StatePath     string
	QuickBin      string
	AWGShowBin    string
	IPTablesBin   string
	IPBin         string
	ExternalIface string
	Runner        Runner
	Log           *slog.Logger

	mu      sync.Mutex
	lastErr error
}

// New создаёт контроллер с дефолтным slog-логгером.
//
// Параметры подбираются из config.Config; если cfg нулевой, читаются env-переменные.
func New(cfg config.Config) *Controller {
	logger := slog.Default().With("component", "netctl")
	return &Controller{
		DataDir:       cfg.DataDir,
		StatePath:     filepath.Join(cfg.DataDir, "net-policy-state.json"),
		QuickBin:      cfg.WGQuickBin,
		AWGShowBin:    cfg.AWGShowBin,
		IPTablesBin:   cfg.IPTablesBin,
		IPBin:         cfg.IPBin,
		ExternalIface: cfg.ExternalIface,
		Runner:        defaultRunner(),
		Log:           logger,
	}
}

func (c *Controller) LastError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr
}

func (c *Controller) setErr(err error) {
	c.mu.Lock()
	c.lastErr = err
	c.mu.Unlock()
}

// Apply — основной вход: пересобрать конфиги интерфейсов, выкатить policy routing и NAT.
//
// Шаги:
//  1. Откат предыдущих ip rule/route и iptables-правил.
//  2. Снятие предыдущих интерфейсов awg/wg-quick.
//  3. Запись свежих конфигов и подъём интерфейсов.
//  4. Применение правил policy routing и NAT с сохранением state.
func (c *Controller) Apply(ctx context.Context, st *store.Store) error {
	prevPol, _ := c.loadState()
	if err := c.undo(ctx, prevPol); err != nil {
		c.setErr(err)
		return err
	}
	prevNAT, _ := c.loadNATState()
	c.undoNAT(ctx, prevNAT)

	if err := c.teardownWireGuard(ctx); err != nil {
		c.setErr(err)
		return err
	}

	wgPaths, err := c.syncWireGuard(ctx, st)
	if err != nil {
		c.setErr(err)
		return err
	}
	if runtime.GOOS == "linux" {
		if err := c.saveWGRuntime(&wgRuntimeState{ConfigPaths: wgPaths}); err != nil {
			c.setErr(err)
			return err
		}
	}

	tunnels, err := st.ListEgressTunnels(ctx)
	if err != nil {
		c.setErr(err)
		return err
	}
	peers, err := st.ListPeers(ctx)
	if err != nil {
		c.setErr(err)
		return err
	}
	sys, err := st.GetSystemSettings(ctx)
	if err != nil {
		c.setErr(err)
		return err
	}

	extIface, err := c.detectExternalInterface(ctx, sys.ExternalInterface)
	if err != nil {
		c.Log.Warn("nat: external interface detection failed; direct peers won't get NAT", "err", err)
	}

	var next PolicyState
	next.Version = 1
	var natRules []NATRule

	tunnelByID := make(map[int64]domain.EgressTunnel)
	for _, t := range tunnels {
		tunnelByID[t.ID] = t
		if !t.Enabled {
			continue
		}
		tbl := domain.RoutingTableID(t.ID)
		if _, err := c.Runner.Run(ctx, c.IPBin, "route", "replace", "default", "dev", t.InterfaceName, "table", strconv.Itoa(tbl)); err != nil {
			c.setErr(err)
			return err
		}
		next.Routes = append(next.Routes, RouteEntry{Table: tbl, Dev: t.InterfaceName})
	}

	for _, p := range peers {
		if !p.Enabled {
			continue
		}
		from, ok := PeerRuleSource(p.AllowedIPs)
		if !ok {
			c.Log.Warn("netctl: skip peer with invalid allowed_ips", "peer_id", p.ID, "allowed_ips", p.AllowedIPs)
			continue
		}

		switch p.EgressType {
		case domain.EgressDirect:
			if extIface != "" {
				natRules = append(natRules, makeMasqueradeRule(from, extIface))
			}
		case domain.EgressViaTunnel:
			if p.EgressTunnelID == nil {
				continue
			}
			tid := *p.EgressTunnelID
			tun, ok := tunnelByID[tid]
			if !ok || !tun.Enabled {
				if sys.TunnelOfflinePolicy == domain.TunnelOfflineBlock {
					err := fmt.Errorf("peer %d references unavailable tunnel %d (policy=block)", p.ID, tid)
					c.setErr(err)
					return err
				}
				c.Log.Warn("netctl: skip peer (tunnel offline, policy=ignore)", "peer_id", p.ID, "tunnel_id", tid)
				continue
			}
			tbl := domain.RoutingTableID(tid)
			pref := domain.RulePreference(p.ID)
			if _, err := c.Runner.Run(ctx, c.IPBin, "rule", "add", "from", from, "table", strconv.Itoa(tbl), "pref", strconv.Itoa(pref)); err != nil {
				c.setErr(err)
				return err
			}
			next.Rules = append(next.Rules, RuleEntry{Pref: pref, From: from, Table: tbl})
			natRules = append(natRules, makeMasqueradeRule(from, tun.InterfaceName))
		}
	}

	if err := c.applyNAT(ctx, natRules); err != nil {
		c.setErr(err)
		return err
	}
	if err := c.saveNATState(&natState{Version: 1, Rules: natRules}); err != nil {
		c.setErr(err)
		return err
	}
	if err := c.saveState(&next); err != nil {
		c.setErr(err)
		return err
	}
	c.setErr(nil)
	c.Log.Info("netctl: apply ok", "ip_rules", len(next.Rules), "ip_routes", len(next.Routes), "nat_rules", len(natRules))
	return nil
}

func (c *Controller) undo(ctx context.Context, prev *PolicyState) error {
	if prev == nil {
		return nil
	}
	if len(prev.Rules) == 0 && len(prev.Routes) == 0 {
		return nil
	}
	var errs []error
	for _, r := range prev.Rules {
		if _, err := c.Runner.Run(ctx, c.IPBin, "rule", "del", "pref", strconv.Itoa(r.Pref)); err != nil {
			errs = append(errs, err)
		}
	}
	seen := make(map[int]struct{})
	for _, rt := range prev.Routes {
		if _, ok := seen[rt.Table]; ok {
			continue
		}
		seen[rt.Table] = struct{}{}
		if _, err := c.Runner.Run(ctx, c.IPBin, "route", "flush", "table", strconv.Itoa(rt.Table)); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func (c *Controller) loadState() (*PolicyState, error) {
	b, err := os.ReadFile(c.StatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s PolicyState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Controller) saveState(s *PolicyState) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.StatePath, b, 0o600)
}

// IngressStatus возвращает рантайм-данные пиров (handshake, rx, tx) по public_key.
func (c *Controller) IngressStatus(ctx context.Context, ifaceName string) (map[string]domain.PeerStatus, error) {
	if runtime.GOOS != "linux" || ifaceName == "" {
		return nil, nil
	}
	dump, err := awgshow.Show(ctx, c.Runner, c.AWGShowBin, ifaceName)
	if err != nil {
		return nil, err
	}
	out := make(map[string]domain.PeerStatus, len(dump.Peers))
	for _, p := range dump.Peers {
		out[p.PublicKey] = domain.PeerStatus{
			PublicKey:         p.PublicKey,
			Endpoint:          p.Endpoint,
			LatestHandshake:   p.LatestHandshake,
			TransferRxBytes:   p.TransferRxBytes,
			TransferTxBytes:   p.TransferTxBytes,
			PersistentKeepAlv: p.PersistentKeepAlv,
		}
	}
	return out, nil
}

// EgressStatus агрегирует первый peer (это удалённый сервер) для каждого исходящего туннеля.
func (c *Controller) EgressStatus(ctx context.Context, ifaceName string) domain.EgressTunnelStatus {
	st := domain.EgressTunnelStatus{}
	if runtime.GOOS != "linux" || ifaceName == "" {
		return st
	}
	dump, err := awgshow.Show(ctx, c.Runner, c.AWGShowBin, ifaceName)
	if err != nil {
		st.LastError = err.Error()
		return st
	}
	st.InterfaceUp = true
	if len(dump.Peers) > 0 {
		p := dump.Peers[0]
		st.LatestHandshake = p.LatestHandshake
		st.TransferRxBytes = p.TransferRxBytes
		st.TransferTxBytes = p.TransferTxBytes
	}
	return st
}

// Status — сводный summary для GET /system/status.
func (c *Controller) Status() map[string]any {
	pr := map[string]any{"os": runtime.GOOS}
	if runtime.GOOS == "linux" {
		pr["backend"] = "linux-ip"
	} else {
		pr["backend"] = "noop"
	}
	c.mu.Lock()
	err := c.lastErr
	c.mu.Unlock()
	if err != nil {
		pr["last_error"] = err.Error()
	}
	wg := map[string]any{
		"quick_bin":  c.QuickBin,
		"awg_bin":    c.AWGShowBin,
		"config_dir": filepath.Join(c.DataDir, "wireguard"),
	}
	if runtime.GOOS != "linux" {
		wg["note"] = "wg-quick runs only on linux"
	}
	return map[string]any{"policy_routing": pr, "wireguard": wg}
}
