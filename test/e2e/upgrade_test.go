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
	"fmt"
	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	testutil "ravendb-operator/test/utils"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpgrade_62_71_happy_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	const toImage = "ravendb/ravendb:7.1.3-ubuntu.22.04-x64"
	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "upgrade-62-71-happy",
		Namespace: testutil.DefaultNS,
	})

	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	require.NoError(t,
		ExtractServerCertToTmp(t.Context(), testutil.DefaultNS, "ravendb-a-0", "", "/ravendb/certs/server.pfx", ""),
		"extract pem/key in pod",
	)

	require.NoError(t,
		CreateDatabaseRF3(t.Context(), testutil.DefaultNS, "ravendb-a-0", "", "e2e_db"),
		"failed to create RF3 DB",
	)

	testutil.PatchSpecImage(t, cli, key, toImage)

	testutil.WaitPodImage(t, cli, testutil.DefaultNS, "ravendb-a-0", toImage, timeout)
	t.Logf("ravendb-a-0 now running %s", toImage)

	testutil.WaitPodImage(t, cli, testutil.DefaultNS, "ravendb-b-0", toImage, timeout)
	t.Logf("ravendb-b-0 now running %s", toImage)

	testutil.WaitPodImage(t, cli, testutil.DefaultNS, "ravendb-c-0", toImage, timeout)
	t.Logf("ravendb-c-0 now running %s", toImage)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)
	t.Logf("cluster marked as upgraded and healthy ConditionReady=True")

	expectedEvents := []string{
		"node A - pre-step/node_alive passed",
		"node A - post-step/node_alive passed",
		"node A - pre-step/cluster_connectivity passed",
		"node A - post-step/cluster_connectivity passed",
		"node A - pre-step/db_groups_available_excluding_target passed",
		"node A - post-step/db_groups_available_excluding_target passed",

		"node B - pre-step/node_alive passed",
		"node B - post-step/node_alive passed",
		"node B - pre-step/cluster_connectivity passed",
		"node B - post-step/cluster_connectivity passed",
		"node B - pre-step/db_groups_available_excluding_target passed",
		"node B - post-step/db_groups_available_excluding_target passed",

		"node C - pre-step/node_alive passed",
		"node C - post-step/node_alive passed",
		"node C - pre-step/cluster_connectivity passed",
		"node C - post-step/cluster_connectivity passed",
		"node C - pre-step/db_groups_available_excluding_target passed",
		"node C - post-step/db_groups_available_excluding_target passed",
	}

	testutil.RequireContainsAllEventually(
		t,
		func() (string, error) { return testutil.OperatorEventsTSVAll(t.Context()) },
		expectedEvents,
		30*time.Second, //let some time for events to appear
	)

	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(t.Context(), key, cur))
	ready, ok := testutil.GetCondition(cur, ravendbv1alpha1.ConditionReady)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1alpha1.ReasonCompleted), ready.Reason)

	RequirePodsRavenVersion(
		t,
		testutil.DefaultNS,
		[]string{"ravendb-a-0", "ravendb-b-0", "ravendb-c-0"},
		"7.1.3",
		20*time.Second,
	)
}

func TestUpgrade_62_71_pre_cluster_conn_fail_on_a_bc_b_down_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	const toImage = "ravendb/ravendb:7.1.3-ubuntu.22.04-x64"

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "upgrade-62_71_pre_cluster_conn_fail_on_a_bc_b_down",
		Namespace: testutil.DefaultNS,
	})

	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	// sabotage node B cert to cause failure
	ctx := context.Background()
	_, err := testutil.RunKubectl(ctx,
		"-n", testutil.DefaultNS,
		"patch", "secret", "ravendb-certs-b",
		"--type=json",
		"-p", `[{"op":"replace","path":"/data/server.pfx","value":"Ym9ndXM="}]`,
	)
	require.NoError(t, err, "patch secret ravendb-certs-b failed")

	_, err = testutil.RunKubectl(ctx, "-n", testutil.DefaultNS, "delete", "pod", "ravendb-b-0", "--wait=false")
	require.NoError(t, err, "delete pod ravendb-b-0 failed")

	testutil.PatchSpecImage(t, cli, key, toImage)
	fetch := func() (string, error) { return testutil.OperatorEventsTSVAll(t.Context()) }

	const aClusterConnStart = "node A - pre-step/cluster_connectivity started"
	const aClusterConnTimeout = "node A - pre-step/cluster_connectivity blocked" // we won't wait here for the full timeout
	_ = testutil.WaitForEventSubstring(t, fetch, aClusterConnStart, 20*time.Second)

	eventsTSV := testutil.WaitForEventSubstring(t, fetch, aClusterConnTimeout, 40*time.Second)

	testutil.RequireNotContainsAny(t, eventsTSV,
		"node A - pre-step/db_groups_available_excluding_target passed",
		"node A - post-step/node_alive passed",
		"node A - post-step/cluster_connectivity passed",
		"node A - post-step/db_groups_available_excluding_target passed",
	)

	// skipping b because the pod is broken
	RequirePodsRavenVersion(
		t,
		testutil.DefaultNS,
		[]string{"ravendb-a-0", "ravendb-c-0"},
		"6.2.9",
		20*time.Second,
	)
}

func TestUpgrade_62_71_degraded_db_placement_on_a_c_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	const (
		toImage = "ravendb/ravendb:7.1.3-ubuntu.22.04-x64"
		dbName  = "my_db"
		ns      = testutil.DefaultNS
		podA    = "ravendb-a-0"
		podB    = "ravendb-b-0"
		podC    = "ravendb-c-0"
	)
	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "upgrade-62_71_degraded_db_placement_on_a_c",
		Namespace: ns,
	})

	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	require.NoError(t, ExtractServerCertToTmp(t.Context(), ns, podA, "", "/ravendb/certs/server.pfx", ""), "extract pem/key")
	require.NoError(t, CreateDatabaseRF3(t.Context(), ns, podA, "", dbName), "create RF3 DB")

	require.NoError(t, SabotageDatabase(t.Context(), ns, podA, "", dbName), "sabotage A")
	require.NoError(t, SabotageDatabase(t.Context(), ns, podC, "", dbName), "sabotage C")
	_, _ = testutil.RunKubectl(t.Context(), "-n", ns, "delete", "pod", "ravendb-a-0", "--wait=false")
	_, _ = testutil.RunKubectl(t.Context(), "-n", ns, "delete", "pod", "ravendb-c-0", "--wait=false")

	time.Sleep(15 * time.Second) // let topology stablizie
	testutil.PatchSpecImage(t, cli, key, toImage)
	fetch := func() (string, error) { return testutil.OperatorEventsTSVAll(t.Context()) }

	testutil.WaitForEventSubstring(t, fetch, "node A - pre-step/node_alive passed", 30*time.Second)
	testutil.WaitForEventSubstring(t, fetch, "node A - pre-step/cluster_connectivity passed", 30*time.Second)
	testutil.WaitForEventSubstring(t, fetch, "node A - pre-step/db_groups_available_excluding_target passed", 30*time.Second)

	testutil.WaitForEventSubstring(t, fetch, "node B - pre-step/node_alive passed", 240*time.Second)
	testutil.WaitForEventSubstring(t, fetch, "node B - pre-step/cluster_connectivity passed", 120*time.Second)

	eventsTSV := testutil.RequireContainsAnyEventually(
		t,
		fetch,
		120*time.Second,
		"node B - pre-step/db_groups_available_excluding_target blocked",
		"node B - pre-step/db_groups_available_excluding_target fail",
		"node B - pre-step/db_groups_available_excluding_target timeout",
	)

	testutil.RequireContainsAny(t, eventsTSV,
		"node B - pre-step/db_groups_available_excluding_target blocked",
		"node B - pre-step/db_groups_available_excluding_target fail",
		"node B - pre-step/db_groups_available_excluding_target timeout",
	)

	testutil.RequireNotContainsAny(t, eventsTSV,
		"node B - post-step/node_alive passed",
		"node B - post-step/cluster_connectivity passed",
		"node B - post-step/db_groups_available_excluding_target passed",
		"node C - pre-step/node_alive started",
	)

	RequirePodsRavenVersion(
		t,
		testutil.DefaultNS,
		[]string{"ravendb-a-0"},
		"7.1.3",
		20*time.Second,
	)

	RequirePodsRavenVersion(
		t,
		testutil.DefaultNS,
		[]string{"ravendb-c-0"},
		"6.2.9",
		20*time.Second,
	)
}

func RequirePodsRavenVersion(t *testing.T, ns string, pods []string, expected string, perPodTimeout time.Duration) {
	t.Helper()
	const ravenBin = "/usr/lib/ravendb/server/Raven.Server"

	for _, pod := range pods {
		ctx, cancel := context.WithTimeout(t.Context(), perPodTimeout)
		out, err := testutil.ExecInPodCapture(ctx, ns, pod, "", ravenBin, "--version")
		cancel()
		require.NoError(t, err, "exec failed on %s: %s", pod, out)
		require.Contains(t, out, expected, "version mismatch on %s\noutput:\n%s", pod, strings.TrimSpace(out))
	}
}

func ExtractServerCertToTmp(ctx context.Context, ns, pod, container, pfxPath, pfxPass string) error {
	if pfxPath == "" {
		pfxPath = "/ravendb/certs/server.pfx"
	}
	cmd := []string{
		"sh", "-lc", fmt.Sprintf(`
PFX=%q
PASS=%q
openssl pkcs12 -in "$PFX" -clcerts -nokeys -out /tmp/cluster.server.certificate.pem -legacy -passin pass:$PASS
openssl pkcs12 -in "$PFX" -nocerts -nodes -out /tmp/cluster.server.certificate.key -legacy -passin pass:$PASS
`, pfxPath, pfxPass),
	}
	_, err := testutil.ExecInPod(ctx, ns, pod, container, cmd...)
	return err
}

func CreateDatabaseRF3(ctx context.Context, ns, pod, container, dbName string) error {
	payload := fmt.Sprintf(`{"DatabaseName":%q,"ReplicationFactor":3,"Topology":{"Members":["A","B","C"]}}`, dbName)

	cmd := []string{
		"sh", "-lc", fmt.Sprintf(`cat <<'JSON' | curl --fail -sS \
  --cert /tmp/cluster.server.certificate.pem \
  --key  /tmp/cluster.server.certificate.key \
  -X PUT -H 'Content-Type: application/json' --data-binary @- \
  https://a.ravendbe2e.development.run/admin/databases
%s
JSON`, payload),
	}
	_, err := testutil.ExecInPod(ctx, ns, pod, container, cmd...)
	return err
}

func SabotageDatabase(ctx context.Context, ns, pod, container, db string) error {
	cmd := []string{
		"sh", "-lc", `
set -eu
DB=/var/lib/ravendb/data/Databases/my_db
rm -rf "$DB" || true
install -m000 -D /dev/null "$DB"
`,
	}
	_, err := testutil.ExecInPodCapture(ctx, ns, pod, container, cmd...)
	return err
}
