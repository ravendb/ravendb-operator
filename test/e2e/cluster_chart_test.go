/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package e2e

import (
	"context"
	"testing"
	"time"

	ravendbv1 "ravendb-operator/api/v1"
	testutil "ravendb-operator/test/utils"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestClusterChart_C1_Provision_E2E(t *testing.T) {
	const ns = "ravendb-cluster-chart-c1"

	cli, key := testutil.InstallClusterChart(t, testutil.ClusterChartCase{
		Name:             "clusterchart-c1-provision",
		Namespace:        ns,
		IngressClassName: "nginx",
		Provision:        true,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	cur := &ravendbv1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1.ConditionReady)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1.ReasonCompleted), cond.Reason)
	require.Equal(t, ravendbv1.PhaseRunning, cur.Status.Phase)

	assertExists(t, cli, ctrlclient.ObjectKey{Namespace: ns, Name: testutil.SecretLicense}, &corev1.Secret{})
	assertExists(t, cli, ctrlclient.ObjectKey{Namespace: ns, Name: testutil.SecretClientPFX}, &corev1.Secret{})
	assertExists(t, cli, ctrlclient.ObjectKey{Namespace: ns, Name: testutil.SecretClusterPFX}, &corev1.Secret{})
	assertExists(t, cli, ctrlclient.ObjectKey{Namespace: ns, Name: testutil.SecretCACert}, &corev1.Secret{})
}

func TestClusterChart_C2_BYO_E2E(t *testing.T) {
	const ns = "ravendb-cluster-chart-c2"

	testutil.RecreateTestEnvInNamespace(t, ns)

	cli, key := testutil.InstallClusterChart(t, testutil.ClusterChartCase{
		Name:             "clusterchart-c2-byo",
		Namespace:        ns,
		IngressClassName: "nginx",
		Provision:        false,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	cur := &ravendbv1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1.ConditionReady)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1.ReasonCompleted), cond.Reason)
	require.Equal(t, ravendbv1.PhaseRunning, cur.Status.Phase)
}

func TestClusterChart_C3_Traefik_E2E(t *testing.T) {
	if ingressController != "traefik" {
		t.Skipf("skipping: requires RAVEN_E2E_INGRESS_CONTROLLER=traefik (got %q)", ingressController)
	}

	const ns = "ravendb-cluster-chart-c3"

	cli, key := testutil.InstallClusterChart(t, testutil.ClusterChartCase{
		Name:             "clusterchart-c3-traefik",
		Namespace:        ns,
		IngressClassName: "traefik",
		Provision:        true,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	cur := &ravendbv1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1.ConditionReady)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1.ReasonCompleted), cond.Reason)
	require.Equal(t, ravendbv1.PhaseRunning, cur.Status.Phase)
}

func TestClusterChart_C4_LifecycleUpgradeChart_E2E(t *testing.T) {
	const ns = "ravendb-cluster-chart-c4"

	cli, key := testutil.InstallClusterChart(t, testutil.ClusterChartCase{
		Name:             "clusterchart-c4-lifecycle-upgrade",
		Namespace:        ns,
		IngressClassName: "nginx",
		Provision:        true,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	upgradeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	require.NoError(t, testutil.RunHelm(upgradeCtx,
		"upgrade", key.Name, testutil.ClusterChartPath(),
		"-n", ns,
		"--reuse-values",
		"--wait",
		"--timeout", (5*time.Minute).String(),
		"--set", "spec.env.RAVEN_Logs_Mode=Information",
	))

	testutil.WaitCondition(t, cli, key, ravendbv1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	cur := &ravendbv1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	require.Equal(t, "Information", cur.Spec.Env["RAVEN_Logs_Mode"])
}

func TestClusterChart_C5_LifecycleUninstall_E2E(t *testing.T) {
	const ns = "ravendb-cluster-chart-c5"

	cli, key := testutil.InstallClusterChart(t, testutil.ClusterChartCase{
		Name:             "clusterchart-c5-lifecycle-uninstall",
		Namespace:        ns,
		IngressClassName: "nginx",
		Provision:        true,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	uninstallCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	require.NoError(t, testutil.RunHelm(uninstallCtx,
		"uninstall", key.Name,
		"-n", ns,
		"--wait",
	))

	testutil.WaitObjectDeleted(t, cli, key, 2*time.Minute)
}

// Hybrid: per-node cert Secrets are pre-seeded by the caller (simulating a
// user who manages those themselves via cert-manager / Vault / sealed-secrets)
// while the chart provisions the license and admin client cert from --set-file.
func TestClusterChart_C6_MixedSecrets_E2E(t *testing.T) {
	const ns = "ravendb-cluster-chart-c6"

	testutil.EnsureNamespace(t, ns, time.Minute)
	testutil.SeedNodeCertSecretsForTagsInNamespace(t, ns, 2*time.Minute, "a", "b", "c")

	cli, key := testutil.InstallClusterChart(t, testutil.ClusterChartCase{
		Name:             "clusterchart-c6-mixed-secrets",
		Namespace:        ns,
		IngressClassName: "nginx",
		ProvisionMixed:   true,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	cur := &ravendbv1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1.ConditionReady)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1.ReasonCompleted), cond.Reason)
	require.Equal(t, ravendbv1.PhaseRunning, cur.Status.Phase)

	assertExists(t, cli, ctrlclient.ObjectKey{Namespace: ns, Name: testutil.SecretLicense}, &corev1.Secret{})
	assertExists(t, cli, ctrlclient.ObjectKey{Namespace: ns, Name: testutil.SecretClientPFX}, &corev1.Secret{})
	assertExists(t, cli, ctrlclient.ObjectKey{Namespace: ns, Name: testutil.SecretClusterPFX}, &corev1.Secret{})
	assertExists(t, cli, ctrlclient.ObjectKey{Namespace: ns, Name: testutil.SecretCACert}, &corev1.Secret{})
}
