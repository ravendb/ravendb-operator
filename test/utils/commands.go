package testutil

import (
	"context"
	"os"
	"os/exec"
)

func RunKubectl(ctx context.Context, args ...string) (context.Context, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return ctx, cmd.Run()
}

func RunKind(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "kind", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func RunDocker(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
