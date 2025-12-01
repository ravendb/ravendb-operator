package testutil

import (
	"bytes"
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

func RunHelm(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func ExecInPod(ctx context.Context, ns, pod, container string, cmd ...string) (context.Context, error) {
	args := []string{"-n", ns, "exec", pod}
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--")
	args = append(args, cmd...)
	return RunKubectl(ctx, args...)
}

func ExecInPodCapture(ctx context.Context, ns, pod, container string, cmd ...string) (string, error) {
	args := []string{"-n", ns, "exec", pod}
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--")
	args = append(args, cmd...)

	c := exec.CommandContext(ctx, "kubectl", args...)
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	err := c.Run()
	return out.String(), err
}

func RunKubectlCapture(ctx context.Context, args ...string) (string, error) {
	c := exec.CommandContext(ctx, "kubectl", args...)
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	err := c.Run()
	return out.String(), err
}
