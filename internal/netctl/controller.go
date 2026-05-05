package netctl

import (
	"context"
	"encoding/json"
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
	c.undo(ctx, prevPol)
	prevNAT, _ := c.loadNATState()
	c.undoNAT(ctx, prevNAT)

	if err := c.teardownWireGuard(ctx); err != nil {
		c.Log.Warn("netctl: teardown wireguard failed", "err", err)
		c.setErr(err)
		return err
	}

	wgPaths, syncErr := c.syncWireGuard(ctx, st)
	// Сохраняем state с теми путями, что успешно поднялись, даже если
	// syncWireGuard вернул агрегатную ошибку — иначе teardownWireGuard
	// при следующем Apply не снесёт уже поднятые интерфейсы.
	if runtime.GOOS == "linux" && len(wgPaths) > 0 {
		if err := c.saveWGRuntime(&wgRuntimeState{ConfigPaths: wgPaths}); err != nil {
			c.setErr(err)
			return err
		}
	}
	if syncErr != nil {
		c.Log.Warn("netctl: sync wireguard failed", "err", syncErr)
		c.setErr(syncErr)
		return syncErr
	}

	upstreams, err := st.ListUpstreamTunnels(ctx)
	if err != nil {
		c.setErr(err)
		return err
	}
	clients, err := st.ListClients(ctx)
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
		c.Log.Warn("nat: external interface detection failed; direct clients won't get NAT", "err", err)
	}

	var next PolicyState
	next.Version = 1
	var natRules []NATRule

	upstreamByID := make(map[int64]domain.UpstreamTunnel)
	for _, t := range upstreams {
		upstreamByID[t.ID] = t
		if !t.Enabled {
			continue
		}
		tbl := domain.RoutingTableID(t.ID)
		if _, err := c.Runner.Run(ctx, c.IPBin, "route", "replace", "default", "dev", t.InterfaceName, "table", strconv.Itoa(tbl)); err != nil {
			c.Log.Warn("netctl: ip route replace failed", "iface", t.InterfaceName, "table", tbl, "err", err)
			c.setErr(err)
			return err
		}
		next.Routes = append(next.Routes, RouteEntry{Table: tbl, Dev: t.InterfaceName})
	}

	for _, cl := range clients {
		if !cl.Enabled {
			continue
		}
		from, ok := PeerRuleSource(cl.AllowedIPs)
		if !ok {
			c.Log.Warn("netctl: skip client with invalid allowed_ips", "client_id", cl.ID, "allowed_ips", cl.AllowedIPs)
			continue
		}

		switch cl.UpstreamType {
		case domain.UpstreamDirect:
			if extIface != "" {
				natRules = append(natRules, makeMasqueradeRule(from, extIface))
			}
		case domain.UpstreamViaTunnel:
			if cl.UpstreamTunnelID == nil {
				continue
			}
			tid := *cl.UpstreamTunnelID
			tun, ok := upstreamByID[tid]
			if !ok || !tun.Enabled {
				if sys.TunnelOfflinePolicy == domain.TunnelOfflineBlock {
					err := fmt.Errorf("client %d references unavailable upstream tunnel %d (policy=block)", cl.ID, tid)
					c.setErr(err)
					return err
				}
				c.Log.Warn("netctl: skip client (upstream offline, policy=ignore)", "client_id", cl.ID, "upstream_id", tid)
				continue
			}
			tbl := domain.RoutingTableID(tid)
			pref := domain.RulePreference(cl.ID)
			if _, err := c.Runner.Run(ctx, c.IPBin, "rule", "add", "from", from, "table", strconv.Itoa(tbl), "pref", strconv.Itoa(pref)); err != nil {
				c.Log.Warn("netctl: ip rule add failed", "from", from, "table", tbl, "pref", pref, "err", err)
				c.setErr(err)
				return err
			}
			next.Rules = append(next.Rules, RuleEntry{Pref: pref, From: from, Table: tbl})
			natRules = append(natRules, makeMasqueradeRule(from, tun.InterfaceName))
		}
	}

	if err := c.applyNAT(ctx, natRules); err != nil {
		c.Log.Warn("netctl: apply NAT failed", "rules", len(natRules), "err", err)
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

// undo откатывает policy-routing-правила и таблицы предыдущего применения.
//
// Реализация — best-effort: ошибки `ip rule del` / `ip route flush` НЕ
// прерывают Apply и НЕ возвращаются вверх. На свежем netns (после
// `docker compose up -d --force-recreate`, рестарта хоста или потери
// сетевых неймспейсов) обе команды легитимно вернут exit 2 / 254
// ("FIB table does not exist", "RTNETLINK answers: No such file..."),
// и это нормальное состояние "уже очищено". Любые прочие ошибки тоже
// не должны блокировать применение свежей конфигурации — поэтому только
// логируем их и идём дальше.
func (c *Controller) undo(ctx context.Context, prev *PolicyState) {
	if prev == nil {
		return
	}
	if len(prev.Rules) == 0 && len(prev.Routes) == 0 {
		return
	}
	for _, r := range prev.Rules {
		if _, err := c.Runner.Run(ctx, c.IPBin, "rule", "del", "pref", strconv.Itoa(r.Pref)); err != nil {
			c.Log.Debug("netctl: undo ip rule del (already gone or transient)", "pref", r.Pref, "err", err)
		}
	}
	seen := make(map[int]struct{})
	for _, rt := range prev.Routes {
		if _, ok := seen[rt.Table]; ok {
			continue
		}
		seen[rt.Table] = struct{}{}
		if _, err := c.Runner.Run(ctx, c.IPBin, "route", "flush", "table", strconv.Itoa(rt.Table)); err != nil {
			c.Log.Debug("netctl: undo ip route flush (already gone or transient)", "table", rt.Table, "err", err)
		}
	}
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

// IngressStatus возвращает runtime-данные клиентов на входном AWG-интерфейсе по public_key.
func (c *Controller) IngressStatus(ctx context.Context, ifaceName string) (map[string]domain.ClientStatus, error) {
	if runtime.GOOS != "linux" || ifaceName == "" {
		return nil, nil
	}
	dump, err := awgshow.Show(ctx, c.Runner, c.AWGShowBin, ifaceName)
	if err != nil {
		return nil, err
	}
	out := make(map[string]domain.ClientStatus, len(dump.Peers))
	for _, p := range dump.Peers {
		out[p.PublicKey] = domain.ClientStatus{
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

// UpstreamStatus собирает первый peer (удалённый AWG-сервер) для каждого upstream-туннеля.
func (c *Controller) UpstreamStatus(ctx context.Context, ifaceName string) domain.UpstreamStatus {
	st := domain.UpstreamStatus{}
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
