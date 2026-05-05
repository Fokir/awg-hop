//go:build linux

package netctl

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, err error)
}

func defaultRunner() Runner { return execRunner{} }

type execRunner struct{}

// awgQuickUserspaceFallback — путь к userspace-реализации AmneziaWG.
// На хостах, где загружен kernel-модуль `wireguard` (например, рядом с wg-easy),
// awg-quick по умолчанию пытается использовать kernel-mode и падает на
// AWG-параметрах (Jc/Jmin/Jmax/S1/S2/H1..H4). Принудительно указываем userspace,
// чтобы поведение не зависело от состояния хост-ядра.
const awgQuickUserspaceFallback = "/usr/local/bin/amneziawg-go"

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if filepath.Base(name) == "awg-quick" {
		env := os.Environ()
		if os.Getenv("WG_QUICK_USERSPACE_IMPLEMENTATION") == "" {
			if _, statErr := os.Stat(awgQuickUserspaceFallback); statErr == nil {
				env = append(env, "WG_QUICK_USERSPACE_IMPLEMENTATION="+awgQuickUserspaceFallback)
			}
		}
		cmd.Env = env
	}

	err := cmd.Run()
	stdout := out.String()
	if err != nil {
		return stdout, &cmdError{name: name, args: args, output: stdout, err: err}
	}
	return stdout, nil
}

// cmdError делает err.Error() информативным: содержит имя команды,
// код возврата и захваченный stdout+stderr. Без этого
// в API/UI и логах оставалось голое "exit status 1".
type cmdError struct {
	name   string
	args   []string
	output string
	err    error
}

func (e *cmdError) Error() string {
	cmdline := e.name
	if len(e.args) > 0 {
		cmdline += " " + strings.Join(e.args, " ")
	}
	out := strings.TrimRight(e.output, "\n")
	if out == "" {
		return fmt.Sprintf("%s: %s", cmdline, e.err)
	}
	return fmt.Sprintf("%s: %s: %s", cmdline, e.err, out)
}

func (e *cmdError) Unwrap() error { return e.err }
