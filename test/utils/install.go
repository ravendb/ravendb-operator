package testutil

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	metallbFilePath       = "test/e2e/manifests/metallb-native.yaml"
	metallbConfigFilePath = "test/e2e/manifests/metallb-config.yaml"
	nginxIngressFilePath  = "test/e2e/manifests/nginx-ingress-ravendb.yaml"
)

func ApplyManifest(path string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		if strings.HasPrefix(path, "https://") {
			return RunKubectl(ctx, "apply", "-f", path)
		}
		return RunKubectl(ctx, "apply", "-f", PathFromRoot(path))
	}
}

func ApplyKustomize(path string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		return RunKubectl(ctx, "apply", "-k", PathFromRoot(path))
	}
}

func ApplyCRDsFromDir(dir string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		return RunKubectl(ctx, "apply", "-f", PathFromRoot(dir))
	}
}

func InstallNodeRBAC(ns, basePath string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		files := []string{
			filepath.Join(basePath, "ravendb-node-sa.yaml"),
			filepath.Join(basePath, "ravendb-node-role.yaml"),
			filepath.Join(basePath, "ravendb-node-rolebinding.yaml"),
		}
		for _, f := range files {
			if _, err := RunKubectl(ctx, "apply", "-f", PathFromRoot(f), "-n", ns); err != nil {
				return ctx, err
			}
		}
		return ctx, nil
	}
}

func BuildAndLoadOperator(image, dockerfile, repoRoot string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		if err := RunDocker(ctx, "build", "-f", PathFromRoot(dockerfile), "-t", image, repoRoot); err != nil {
			return ctx, fmt.Errorf("docker build: %w", err)
		}
		if err := RunKind(ctx, "load", "docker-image", image, "--name", "ravendb"); err != nil {
			return ctx, fmt.Errorf("kind load: %w", err)
		}
		return ctx, nil
	}
}

func SetDeploymentImage(ns, deploy, container, image string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		_, err := RunKubectl(ctx, "-n", ns, "set", "image", "deploy/"+deploy, container+"="+image)
		return ctx, err
	}
}

func WaitForCRDEstablished(crd string, timeout time.Duration) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		return RunKubectl(ctx, "wait", "--for=condition=Established", "--timeout="+timeout.String(), "crd/"+crd)
	}
}

func WaitForDeployment(name, ns string, timeout time.Duration) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		return RunKubectl(ctx, "-n", ns, "rollout", "status", "deploy/"+name, "--timeout="+timeout.String())
	}
}

func WaitForIngressControllerReady(timeout time.Duration) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		if _, err := RunKubectl(ctx, "-n", "ingress-nginx", "rollout", "status", "deploy/ingress-nginx-controller", "--timeout="+timeout.String()); err != nil {
			return ctx, err
		}
		if _, err := RunKubectl(ctx, "-n", "ingress-nginx", "wait", "--for=condition=Ready", "--timeout="+timeout.String(), "pods", "-l", "app.kubernetes.io/name=ingress-nginx,app.kubernetes.io/component=controller"); err != nil {
			return ctx, err
		}
		return ctx, nil
	}
}

func WaitForIngressAdmissionReady(timeout time.Duration) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		if _, err := RunKubectl(ctx, "-n", "ingress-nginx", "wait", "--for=condition=complete", "--timeout="+timeout.String(), "job/ingress-nginx-admission-create"); err != nil {
			return ctx, err
		}
		if _, err := RunKubectl(ctx, "-n", "ingress-nginx", "wait", "--for=condition=complete", "--timeout="+timeout.String(), "job/ingress-nginx-admission-patch"); err != nil {
			return ctx, err
		}
		time.Sleep(2 * time.Second)
		return ctx, nil
	}
}

func DisableMetalLB(t THelper) {
	t.Helper()
	ctx := context.Background()

	_, _ = RunKubectl(ctx, "delete", "namespace", "metallb-system", "--ignore-not-found")

	deadline := time.Now().Add(90 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for metallb-system namespace to be deleted")
		}
		_, err := RunKubectl(ctx, "get", "ns", "metallb-system", "-o", "name")
		if err != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	_, _ = RunKubectl(ctx, "-n", "ingress-nginx", "delete", "svc", "ingress-nginx-controller", "--ignore-not-found")
	_, err := RunKubectl(ctx, "apply", "-f", PathFromRoot(nginxIngressFilePath))
	if err != nil {
		t.Fatalf("re-applying nginx ingress failed: %v", err)
	}
	_, err = RunKubectl(ctx, "-n", "ingress-nginx", "rollout", "status", "deploy/ingress-nginx-controller", "--timeout=120s")
	if err != nil {
		t.Fatalf("waiting for nginx controller rollout: %v", err)
	}
}

func EnableMetalLB(t THelper, controllerNS, metalLBNS string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()

	if _, err := RunKubectl(ctx, "apply", "-f", PathFromRoot(metallbFilePath)); err != nil {
		t.Fatalf("apply metallb: %v", err)
	}
	if _, err := RunKubectl(ctx, "-n", metalLBNS, "rollout", "status", "deploy/controller", "--timeout="+timeout.String()); err != nil {
		t.Fatalf("wait metallb controller: %v", err)
	}
	if _, err := RunKubectl(ctx, "apply", "-f", PathFromRoot(metallbConfigFilePath)); err != nil {
		t.Fatalf("apply metallb config: %v", err)
	}

	time.Sleep(2 * time.Second)
}

type THelper interface {
	Helper()
	Fatalf(format string, args ...any)
}
