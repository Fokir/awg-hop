package wgquick

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Exec минимальный интерфейс запуска команд (совместим с netctl.Runner).
type Exec interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, err error)
}

var ifaceOK = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9._-]{0,14}$`)

// ValidateInterfaceName проверяет имя Linux-интерфейса (до 15 символов).
func ValidateInterfaceName(name string) error {
	name = strings.TrimSpace(name)
	if !ifaceOK.MatchString(name) {
		return fmt.Errorf("invalid interface name %q", name)
	}
	return nil
}

func WireguardDir(dataDir string) string {
	return filepath.Join(dataDir, "wireguard")
}

func EnsureWireguardDir(dataDir string) error {
	return os.MkdirAll(WireguardDir(dataDir), 0o700)
}

// Down игнорирует ошибку (интерфейса может не быть).
func Down(ctx context.Context, ex Exec, quickBin, confAbsPath string) {
	if quickBin == "" {
		quickBin = "wg-quick"
	}
	_, _ = ex.Run(ctx, quickBin, "down", confAbsPath)
}

func Up(ctx context.Context, ex Exec, quickBin, confAbsPath string) error {
	if quickBin == "" {
		quickBin = "wg-quick"
	}
	_, err := ex.Run(ctx, quickBin, "up", confAbsPath)
	return err
}
