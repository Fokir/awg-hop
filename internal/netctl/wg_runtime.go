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
	// Снимаем в обратном порядке (сначала egress, в конце вход — как применяли).
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
	if err := wgquick.ValidateInterfaceName(in.InterfaceName); err != nil {
		return nil, err
	}

	peers, err := st.ListPeers(ctx)
	if err != nil {
		return nil, err
	}

	ingressPath := filepath.Join(wgquick.WireguardDir(c.DataDir), in.InterfaceName+".conf")
	ingressConf := amnezia.BuildIngressServerConf(in, peers)
	if err := os.WriteFile(ingressPath, []byte(ingressConf), 0o600); err != nil {
		return nil, err
	}

	tunnels, err := st.ListEgressTunnels(ctx)
	if err != nil {
		return nil, err
	}

	var paths []string
	paths = append(paths, ingressPath)
	if err := wgquick.Up(ctx, c.Runner, c.QuickBin, ingressPath); err != nil {
		return nil, err
	}

	for _, t := range tunnels {
		if !t.Enabled || strings.TrimSpace(t.ConfigText) == "" {
			continue
		}
		if err := wgquick.ValidateInterfaceName(t.InterfaceName); err != nil {
			return nil, err
		}
		p := filepath.Join(wgquick.WireguardDir(c.DataDir), fmt.Sprintf("egress-%d-%s.conf", t.ID, sanitizeFilename(t.InterfaceName)))
		if err := os.WriteFile(p, []byte(t.ConfigText), 0o600); err != nil {
			return nil, err
		}
		paths = append(paths, p)
		if err := wgquick.Up(ctx, c.Runner, c.QuickBin, p); err != nil {
			return nil, err
		}
	}

	return paths, nil
}

func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	if b.Len() == 0 {
		return "iface"
	}
	if b.Len() > 32 {
		return b.String()[:32]
	}
	return b.String()
}
