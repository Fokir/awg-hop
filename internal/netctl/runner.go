//go:build !linux

package netctl

import (
	"context"
	"log"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, err error)
}

func defaultRunner() Runner { return noopRunner{} }

type noopRunner struct{}

func (noopRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	log.Printf("netctl (noop): %s %v", name, args)
	return "", nil
}
