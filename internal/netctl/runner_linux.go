//go:build linux

package netctl

import (
	"bytes"
	"context"
	"os/exec"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, err error)
}

func defaultRunner() Runner { return execRunner{} }

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}
