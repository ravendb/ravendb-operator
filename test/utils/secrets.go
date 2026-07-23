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
	SecretLicense    = "ravendb-license"
	SecretClientPFX  = "ravendb-client-cert"
	SecretClusterPFX = "ravendb-cert"
	SecretCACert     = "ravendb-ca-cert"
	SecretNodeAPFX   = "ravendb-certs-a"
	SecretNodeBPFX   = "ravendb-certs-b"
	SecretNodeCPFX   = "ravendb-certs-c"
	SecretNodeDPFX   = "ravendb-certs-d"
	SecretNodeEPFX   = "ravendb-certs-e"
	SecretNodeFPFX   = "ravendb-certs-f"
)

const (
	EnvLicensePath    = "E2E_LICENSE_PATH"
	EnvClientPFXPath  = "E2E_CLIENT_PFX_PATH"
	EnvClusterPFXPath = "E2E_CLUSTER_PFX_PATH"
	EnvCACertPath     = "E2E_CA_CERT_PATH"
	EnvNodeAPFXPath   = "E2E_NODE_A_PFX_PATH"
	EnvNodeBPFXPath   = "E2E_NODE_B_PFX_PATH"
	EnvNodeCPFXPath   = "E2E_NODE_C_PFX_PATH"
	EnvNodeDPFXPath   = "E2E_NODE_D_PFX_PATH"
	EnvNodeEPFXPath   = "E2E_NODE_E_PFX_PATH"
	EnvNodeFPFXPath   = "E2E_NODE_F_PFX_PATH"
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

func nodeSecretName(tag string) string {
	return fmt.Sprintf("ravendb-certs-%s", tag)
}

func nodeSecretEnvVar(tag string) string {
	switch tag {
	case "a":
		return EnvNodeAPFXPath
	case "b":
		return EnvNodeBPFXPath
	case "c":
		return EnvNodeCPFXPath
	case "d":
		return EnvNodeDPFXPath
	case "e":
		return EnvNodeEPFXPath
	case "f":
		return EnvNodeFPFXPath
	default:
		return ""
	}
}

func SeedSecrets(t *testing.T) {
	SeedLESecretsForTagsInNamespace(t, DefaultNS, 2*time.Minute, "a", "b", "c")
}

func SeedLESecretsInNamespace(t *testing.T, ns string, timeout time.Duration) {
	t.Helper()
	SeedLESecretsForTagsInNamespace(t, ns, timeout, "a", "b", "c")
}

func seedRun(t *testing.T, timeout time.Duration, f env.Func) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := f(ctx, envconf.New())
	require.NoError(t, err)
}

// SeedServerCertSecretsInNamespace pre-creates the server-side secrets for
// Mode=None: the single cluster server certificate and the CA. Used by the
// mixed chart case, where the chart provisions the license + client cert and
// these are brought pre-created.
//
// The variadic tags are ignored: Mode=None uses one shared cluster cert (with
// SANs for every node), not per-node certificates. The parameter is retained so
// existing callers need not change.
func SeedServerCertSecretsInNamespace(t *testing.T, ns string, timeout time.Duration, _ ...string) {
	t.Helper()
	seedRun(t, timeout, EnsureSecretFromEnvPath(ns, SecretClusterPFX, "server.pfx", EnvClusterPFXPath))
	seedRun(t, timeout, EnsureSecretFromEnvPath(ns, SecretCACert, "ca.crt", EnvCACertPath))
	seedRun(t, timeout, WaitForSecret(SecretClusterPFX, ns, timeout))
	seedRun(t, timeout, WaitForSecret(SecretCACert, ns, timeout))
}

// SeedNodeCertSecretsForTagsInNamespace is retained under its historical name
// for callers; under Mode=None it seeds the shared server-side secrets.
func SeedNodeCertSecretsForTagsInNamespace(t *testing.T, ns string, timeout time.Duration, tags ...string) {
	SeedServerCertSecretsInNamespace(t, ns, timeout, tags...)
}

// SeedLESecretsForTagsInNamespace seeds the full BYO secret set for a Mode=None
// cluster: license, admin client cert, cluster server cert, and CA. (Name kept
// for callers; the suite now runs Mode=None with generated certs.)
func SeedLESecretsForTagsInNamespace(t *testing.T, ns string, timeout time.Duration, _ ...string) {
	t.Helper()

	seedRun(t, timeout, EnsureSecretFromEnvPath(ns, SecretLicense, "license.json", EnvLicensePath))
	seedRun(t, timeout, EnsureSecretFromEnvPath(ns, SecretClientPFX, "client.pfx", EnvClientPFXPath))
	seedRun(t, timeout, EnsureSecretFromEnvPath(ns, SecretClusterPFX, "server.pfx", EnvClusterPFXPath))
	seedRun(t, timeout, EnsureSecretFromEnvPath(ns, SecretCACert, "ca.crt", EnvCACertPath))

	seedRun(t, timeout, WaitForSecret(SecretLicense, ns, timeout))
	seedRun(t, timeout, WaitForSecret(SecretClientPFX, ns, timeout))
	seedRun(t, timeout, WaitForSecret(SecretClusterPFX, ns, timeout))
	seedRun(t, timeout, WaitForSecret(SecretCACert, ns, timeout))
}
