package netctl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"awghop/internal/amnezia"
	"awghop/internal/store"
	"awghop/internal/wgquick"
)

const wgRuntimeStateFile = "wireguard-runtime-state.json"

type wgRuntimeState struct {
	ConfigPaths []string `json:"config_paths"`
}

func (c *Controller) wgRuntimePath() string {
	return filepath.Join(c.DataDir, wgRuntimeStateFile)
}

func (c *Controller) loadWGRuntime() (*wgRuntimeState, error) {
	b, err := os.ReadFile(c.wgRuntimePath())
	if err != nil {
		if os.IsNotExist(err) {
			return &wgRuntimeState{}, nil
		}
		return nil, err
	}
	var s wgRuntimeState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *Controller) saveWGRuntime(s *wgRuntimeState) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.wgRuntimePath(), b, 0o600)
}

func (c *Controller) teardownWireGuard(ctx context.Context) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	prev, err := c.loadWGRuntime()
	if err != nil {
		return err
	}
	// Снимаем в обратном порядке (сначала upstream-интерфейсы, в конце вход — как применяли).
	for i := len(prev.ConfigPaths) - 1; i >= 0; i-- {
		wgquick.Down(ctx, c.Runner, c.QuickBin, prev.ConfigPaths[i])
	}
	return nil
}

func (c *Controller) syncWireGuard(ctx context.Context, st *store.Store) ([]string, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	if err := wgquick.EnsureWireguardDir(c.DataDir); err != nil {
		return nil, err
	}

	in, err := st.GetIngressSettings(ctx)
	if err != nil {
		return nil, err
	}
	// Самолечение для уже забутстрапленных установок: если в БД остались
	// нулевые H1..H4 или пустые S1/S2 — генерим валидные значения и
	// сохраняем, иначе `awg setconf` отвергнет конфиг с Invalid argument.
	if changed, derr := amnezia.EnsureAmneziaDefaults(&in); derr != nil {
		return nil, fmt.Errorf("ensure amnezia defaults: %w", derr)
	} else if changed {
		if uerr := st.UpdateIngressSettings(ctx, in); uerr != nil {
			return nil, fmt.Errorf("persist amnezia defaults: %w", uerr)
		}
		c.Log.Info("netctl: ingress amnezia defaults regenerated and persisted")
	}
	if err := wgquick.ValidateInterfaceName(in.InterfaceName); err != nil {
		return nil, err
	}

	clients, err := st.ListClients(ctx)
	if err != nil {
		return nil, err
	}

	ingressPath := filepath.Join(wgquick.WireguardDir(c.DataDir), in.InterfaceName+".conf")
	ingressConf := amnezia.BuildIngressServerConf(in, clients)
	if err := os.WriteFile(ingressPath, []byte(ingressConf), 0o600); err != nil {
		return nil, err
	}

	upstreams, err := st.ListUpstreamTunnels(ctx)
	if err != nil {
		return nil, err
	}

	var paths []string
	paths = append(paths, ingressPath)
	if err := wgquick.Up(ctx, c.Runner, c.QuickBin, ingressPath); err != nil {
		c.Log.Warn("netctl: awg-quick up (ingress) failed", "iface", in.InterfaceName, "config", ingressPath, "err", err)
		return nil, err
	}

	for _, t := range upstreams {
		if !t.Enabled || strings.TrimSpace(t.ConfigText) == "" {
			continue
		}
		if err := wgquick.ValidateInterfaceName(t.InterfaceName); err != nil {
			return nil, fmt.Errorf("upstream %d (%q) has invalid interface name: %w", t.ID, t.Name, err)
		}
		// awg-quick извлекает имя интерфейса из basename .conf-файла и требует
		// его соответствия Linux IFNAMSIZ (<=15 символов). Поэтому файл должен
		// называться ровно "<interface_name>.conf", без префиксов.
		if t.InterfaceName == in.InterfaceName {
			return nil, fmt.Errorf("upstream %d (%q) interface name %q collides with ingress interface", t.ID, t.Name, t.InterfaceName)
		}
		p := filepath.Join(wgquick.WireguardDir(c.DataDir), t.InterfaceName+".conf")
		if err := os.WriteFile(p, []byte(t.ConfigText), 0o600); err != nil {
			return nil, err
		}
		paths = append(paths, p)
		if err := wgquick.Up(ctx, c.Runner, c.QuickBin, p); err != nil {
			c.Log.Warn("netctl: awg-quick up (upstream) failed", "upstream_id", t.ID, "iface", t.InterfaceName, "config", p, "err", err)
			return nil, err
		}
	}

	return paths, nil
}

