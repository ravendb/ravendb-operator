package testutil

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const (
	SecretLicense   = "ravendb-license"
	SecretClientPFX = "ravendb-client-cert"
	SecretNodeAPFX  = "ravendb-certs-a"
	SecretNodeBPFX  = "ravendb-certs-b"
	SecretNodeCPFX  = "ravendb-certs-c"
)

const (
	EnvLicensePath   = "E2E_LICENSE_PATH"
	EnvClientPFXPath = "E2E_CLIENT_PFX_PATH"
	EnvNodeAPFXPath  = "E2E_NODE_A_PFX_PATH"
	EnvNodeBPFXPath  = "E2E_NODE_B_PFX_PATH"
	EnvNodeCPFXPath  = "E2E_NODE_C_PFX_PATH"
)

func EnsureSecretFromEnvPath(ns, name, key, envVar string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		p := os.Getenv(envVar)
		if p == "" {
			return ctx, fmt.Errorf("%s not set (secret %s)", envVar, name)
		}
		return EnsureOpaqueSecretFromFile(ns, name, key, p)(ctx, nil)
	}
}

func EnsureOpaqueSecretFromFile(ns, name, key, filePath string) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		if _, err := os.Stat(filePath); err != nil {
			return ctx, fmt.Errorf("file for secret %s not found: %s", name, filePath)
		}
		_, _ = RunKubectl(ctx, "-n", ns, "delete", "secret", name, "--ignore-not-found")
		return RunKubectl(ctx, "-n", ns, "create", "secret", "generic", name, fmt.Sprintf("--from-file=%s=%s", key, filePath))
	}
}

func WaitForSecret(name, ns string, timeout time.Duration) env.Func {
	return func(ctx context.Context, _ *envconf.Config) (context.Context, error) {
		deadline := time.Now().Add(timeout)
		for {
			_, err := RunKubectl(ctx, "-n", ns, "get", "secret", name, "-o", "name")
			if err == nil {
				return ctx, nil
			}
			if time.Now().After(deadline) {
				return ctx, fmt.Errorf("timeout waiting for secret %s/%s", ns, name)
			}
			time.Sleep(time.Second)
		}
	}
}

func SeedSecrets(t *testing.T) {
	SeedLESecretsInNamespace(t, DefaultNS, 2*time.Minute)
}

func SeedLESecretsInNamespace(t *testing.T, ns string, timeout time.Duration) {
	t.Helper()

	run := func(f env.Func) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		_, err := f(ctx, envconf.New())
		require.NoError(t, err)
	}

	run(EnsureSecretFromEnvPath(ns, SecretLicense, "license.json", EnvLicensePath))
	run(EnsureSecretFromEnvPath(ns, SecretClientPFX, "client.pfx", EnvClientPFXPath))
	run(EnsureSecretFromEnvPath(ns, SecretNodeAPFX, "server.pfx", EnvNodeAPFXPath))
	run(EnsureSecretFromEnvPath(ns, SecretNodeBPFX, "server.pfx", EnvNodeBPFXPath))
	run(EnsureSecretFromEnvPath(ns, SecretNodeCPFX, "server.pfx", EnvNodeCPFXPath))

	run(WaitForSecret(SecretLicense, ns, timeout))
	run(WaitForSecret(SecretClientPFX, ns, timeout))
	run(WaitForSecret(SecretNodeAPFX, ns, timeout))
	run(WaitForSecret(SecretNodeBPFX, ns, timeout))
	run(WaitForSecret(SecretNodeCPFX, ns, timeout))
}
