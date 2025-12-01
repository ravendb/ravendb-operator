package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	metallbFilePath       = "https://raw.githubusercontent.com/metallb/metallb/v0.14.3/config/manifests/metallb-native.yaml"
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

func ApplyManifestWithContext(t THelper, path string) {
	t.Helper()
	ctx := context.Background()
	src := path
	if !strings.HasPrefix(path, "https://") {
		src = PathFromRoot(path)
	}
	out, err := RunKubectl(ctx, "apply", "-f", src)
	if err != nil {
		t.Fatalf("kubectl apply -f %s failed: %v\n%s", src, err, out)
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
			filepath.Join(basePath, "ravendb_ops_rbac.yaml"),
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
		if os.Getenv("RAVEN_OPERATOR_IMAGE_PREBUILT") != "1" {
			if err := RunDocker(ctx, "build", "-f", PathFromRoot(dockerfile), "-t", image, repoRoot); err != nil {
				return ctx, fmt.Errorf("docker build: %w", err)
			}
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
	ApplyManifestWithContext(t, nginxIngressFilePath)
	_, err := RunKubectl(ctx, "-n", "ingress-nginx", "rollout", "status", "deploy/ingress-nginx-controller", "--timeout=120s")
	if err != nil {
		t.Fatalf("waiting for nginx controller rollout: %v", err)
	}
}

func EnableMetalLB(t THelper, controllerNS, metalLBNS string, timeout time.Duration) {
	t.Helper()
	ctx := context.Background()

	_, _ = RunKubectl(ctx, "create", "namespace", metalLBNS)

	ApplyManifestWithContext(t, metallbFilePath)
	if _, err := RunKubectl(ctx, "-n", metalLBNS, "rollout", "status", "deploy/controller", "--timeout="+timeout.String()); err != nil {
		t.Fatalf("wait metallb controller: %v", err)
	}
	ApplyManifestWithContext(t, metallbConfigFilePath)

	time.Sleep(2 * time.Second)
}

type THelper interface {
	Helper()
	Fatalf(format string, args ...any)
}

func Getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func PatchImagePullPolicyIfNotPresent(ns, deploy string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		out, err := RunKubectlCapture(
			ctx,
			"-n", ns,
			"get", "deploy", deploy,
			"-o", "jsonpath={.spec.template.spec.containers[0].imagePullPolicy}",
		)
		if err != nil {
			return ctx, err
		}

		current := strings.TrimSpace(out)
		if current == "IfNotPresent" {
			return ctx, nil
		}

		return RunKubectl(ctx,
			"-n", ns,
			"patch", "deploy", deploy,
			"--type=json",
			"-p", `[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"IfNotPresent"}]`)
	}
}

func DumpDeploymentImage(ns, deploy string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		_, err := RunKubectl(ctx, "-n", ns, "get", "deploy", deploy,
			"-o", `jsonpath={.spec.template.spec.containers[0].image}{"\n"}{.spec.template.spec.containers[0].imagePullPolicy}{"\n"}`)
		return ctx, err
	}
}

func InstallOperatorHelm(release, ns, chartRelPath string, timeout time.Duration) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		chartPath := PathFromRoot(chartRelPath)
		extra := strings.Fields(os.Getenv("RAVEN_E2E_HELM_ARGS"))

		args := []string{
			"upgrade", "--install", release, chartPath,
			"-n", ns,
			"--create-namespace",
			"--wait",
			"--timeout", timeout.String(),
			"--skip-crds",
			// disable crd creation in tests to avoid helm ownership conflicts because the test framework applies the CRDs first
			"--set", "crds.enabled=false",
		}

		repo := os.Getenv("RAVEN_OPERATOR_IMAGE_REPO")
		tag := os.Getenv("RAVEN_OPERATOR_IMAGE_TAG")
		if repo != "" {
			args = append(args, "--set", "controllerManager.image.repository="+repo)
		}
		if tag != "" {
			args = append(args, "--set", "controllerManager.image.tag="+tag)
		}

		args = append(args, extra...)

		if err := RunHelm(ctx, args...); err != nil {
			return ctx, fmt.Errorf("helm install operator: %w", err)
		}
		return ctx, nil
	}
}
