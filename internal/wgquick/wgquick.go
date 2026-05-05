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

// SetInterfaceDirective гарантирует наличие `key = value` в секции [Interface]:
// все существующие строки с этим ключом удаляются, а одна новая вставляется
// сразу после заголовка `[Interface]`. Если секции нет — она создаётся в начале.
//
// Используется, чтобы принудительно проставить `Table = off` для апстрим-конфигов:
// иначе awg-quick при `AllowedIPs = 0.0.0.0/0` сам пытается настроить policy
// routing (fwmark + ip rule + ip route + sysctl), что (а) дублирует логику
// Controller.Apply, (б) падает на `sysctl: permission denied` в Docker, где
// /proc/sys смонтирован read-only.
func SetInterfaceDirective(conf, key, value string) string {
	cleaned := StripInterfaceDirective(conf, key)
	insert := key + " = " + value + "\n"
	lines := splitLinesPreserveCRLF(cleaned)
	for i, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "[Interface]") {
			eol := "\n"
			if strings.HasSuffix(line, "\r\n") {
				eol = "\r\n"
				insert = key + " = " + value + "\r\n"
			}
			if !strings.HasSuffix(line, "\n") {
				lines[i] = line + eol
			}
			out := append([]string{}, lines[:i+1]...)
			out = append(out, insert)
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "")
		}
	}
	return "[Interface]\n" + insert + cleaned
}

// StripInterfaceDirective удаляет указанные ключи из секции [Interface].
// Используется, когда мы поднимаем туннель через wg-quick/awg-quick в
// контейнере без resolvconf/systemd-resolved: строка `DNS = ...` заставит
// wg-quick вызвать `resolvconf`, которого в минимальном образе нет, и
// поднятие интерфейса завершится exit 127. Для апстрим-туннелей системный
// resolver контейнера менять не нужно — DNS-серверы остаются в клиентских
// конфигах (BuildClientConf), которые применяются на устройстве клиента.
//
// Сравнение ключей регистронезависимое; затрагивается только секция
// [Interface] (где-нибудь в [Peer] DNS не встречается, но мы перестраховываемся).
func StripInterfaceDirective(conf string, keys ...string) string {
	if len(keys) == 0 {
		return conf
	}
	want := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		want[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}

	var b strings.Builder
	b.Grow(len(conf))
	inInterface := false
	for _, line := range splitLinesPreserveCRLF(conf) {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"):
			inInterface = strings.EqualFold(trimmed, "[Interface]")
			b.WriteString(line)
		case inInterface && trimmed != "" && !strings.HasPrefix(trimmed, "#"):
			if eq := strings.IndexByte(trimmed, '='); eq > 0 {
				key := strings.ToLower(strings.TrimSpace(trimmed[:eq]))
				if _, drop := want[key]; drop {
					continue
				}
			}
			b.WriteString(line)
		default:
			b.WriteString(line)
		}
	}
	return b.String()
}

func splitLinesPreserveCRLF(s string) []string {
	var out []string
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			if s != "" {
				out = append(out, s)
			}
			return out
		}
		out = append(out, s[:i+1])
		s = s[i+1:]
	}
}
