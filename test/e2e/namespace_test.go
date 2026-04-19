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
	"testing"
	"time"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"
	testutil "ravendb-operator/test/utils"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNamespace_HappyPath_E2E(t *testing.T) {
	cases := []struct {
		name       string
		workloadNS string
	}{
		{
			name:       "custom namespace",
			workloadNS: "ravendb-team-a",
		},
		{
			name:       "legacy ravendb namespace",
			workloadNS: "ravendb",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workloadNS := tc.workloadNS
			testutil.RecreateTestEnvInNamespace(t, workloadNS)
			cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
				Name:      fmt.Sprintf("namespace-%s", workloadNS),
				Namespace: workloadNS,
			})
			testutil.RegisterClusterCleanup(t, cli, key, timeout)

			testutil.WaitCondition(t, cli, key, ravendbv1.ConditionReady, metav1.ConditionTrue, 5*time.Minute, 2*time.Second)

			cur := &ravendbv1.RavenDBCluster{}
			require.NoError(t, cli.Get(context.Background(), key, cur))
			require.Equal(t, workloadNS, cur.Namespace)

			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.Prefix + "a"}, &appsv1.StatefulSet{})
			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.Prefix + "b"}, &appsv1.StatefulSet{})
			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.Prefix + "c"}, &appsv1.StatefulSet{})

			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.Prefix + "a"}, &corev1.Service{})
			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.Prefix + "b"}, &corev1.Service{})
			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.Prefix + "c"}, &corev1.Service{})

			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.RavenDbBootstrapperJob}, &batchv1.Job{})
			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.BootstrapperHookConfigMap}, &corev1.ConfigMap{})
			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.CertHookConfigMap}, &corev1.ConfigMap{})

			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: common.RavenDbNodeServiceAccount}, &corev1.ServiceAccount{})
			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: "ravendb-ops"}, &rbacv1.Role{})
			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: workloadNS, Name: "ravendb-ops-role-binding"}, &rbacv1.RoleBinding{})

			// sanity: operator deployment remains in operator namespace
			assertExists(t, cli, ctrlclient.ObjectKey{Namespace: operatorNS, Name: ctlMgrName}, &appsv1.Deployment{})
		})
	}
}

func TestNamespace_TwoClustersTwoNamespaces_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t)

	const (
		nsA = "ravendb-team-a"
		nsB = "ravendb-team-b"
	)

	testutil.EnsureNamespace(t, nsA, 60*time.Second)
	testutil.EnsureNamespace(t, nsB, 60*time.Second)

	testutil.SeedLESecretsForTagsInNamespace(t, nsA, 2*time.Minute, "a", "b", "c")
	testutil.SeedLESecretsForTagsInNamespace(t, nsB, 2*time.Minute, "d", "e", "f")

	cliA, keyA := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "namespace-two-clusters-a",
		Namespace: nsA,
	})

	testutil.RegisterClusterCleanup(t, cliA, keyA, timeout)

	cliB, keyB := testutil.CreateCluster(t, testutil.BaseClusterLEDEF, testutil.ClusterCase{
		Name:      "namespace-two-clusters-b",
		Namespace: nsB,
	})

	testutil.RegisterClusterCleanup(t, cliB, keyB, timeout)

	testutil.WaitCondition(t, cliA, keyA, ravendbv1.ConditionReady, metav1.ConditionTrue, 8*time.Minute, 2*time.Second)
	testutil.WaitCondition(t, cliB, keyB, ravendbv1.ConditionReady, metav1.ConditionTrue, 8*time.Minute, 2*time.Second)

	curA := &ravendbv1.RavenDBCluster{}
	require.NoError(t, cliA.Get(context.Background(), keyA, curA))
	require.Equal(t, nsA, curA.Namespace)

	curB := &ravendbv1.RavenDBCluster{}
	require.NoError(t, cliB.Get(context.Background(), keyB, curB))
	require.Equal(t, nsB, curB.Namespace)

	// cluster A resources stay in nsA
	assertExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "a"}, &appsv1.StatefulSet{})
	assertExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "b"}, &appsv1.StatefulSet{})
	assertExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "c"}, &appsv1.StatefulSet{})

	assertExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "a"}, &corev1.Service{})
	assertExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "b"}, &corev1.Service{})
	assertExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "c"}, &corev1.Service{})

	// cluster B resources stay in nsB
	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "d"}, &appsv1.StatefulSet{})
	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "e"}, &appsv1.StatefulSet{})
	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "f"}, &appsv1.StatefulSet{})

	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "d"}, &corev1.Service{})
	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "e"}, &corev1.Service{})
	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "f"}, &corev1.Service{})

	// sanity: cluster A objects must not appear in cluster B ns
	assertNotExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "a"}, &appsv1.StatefulSet{})
	assertNotExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "b"}, &appsv1.StatefulSet{})
	assertNotExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "c"}, &appsv1.StatefulSet{})

	// sanity: cluster B objects must not appear in cluster A ns
	assertNotExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "d"}, &appsv1.StatefulSet{})
	assertNotExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "e"}, &appsv1.StatefulSet{})
	assertNotExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "f"}, &appsv1.StatefulSet{})
}

func TestNamespace_DeleteOneCluster_OtherUnaffected_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t)

	nsA := "ravendb-team-a"
	nsB := "ravendb-team-b"

	testutil.EnsureNamespace(t, nsA, timeout)
	testutil.EnsureNamespace(t, nsB, timeout)

	testutil.SeedLESecretsForTagsInNamespace(t, nsA, timeout, "a", "b", "c")
	testutil.SeedLESecretsForTagsInNamespace(t, nsB, timeout, "d", "e", "f")

	cliA, keyA := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "cluster-a",
		Namespace: nsA,
	})

	cliB, keyB := testutil.CreateCluster(t, testutil.BaseClusterLEDEF, testutil.ClusterCase{
		Name:      "cluster-b",
		Namespace: nsB,
	})

	testutil.RegisterClusterCleanup(t, cliA, keyA, timeout)
	testutil.RegisterClusterCleanup(t, cliB, keyB, timeout)

	testutil.WaitCondition(t, cliA, keyA, ravendbv1.ConditionReady, metav1.ConditionTrue, 8*time.Minute, 2*time.Second)
	testutil.WaitCondition(t, cliB, keyB, ravendbv1.ConditionReady, metav1.ConditionTrue, 8*time.Minute, 2*time.Second)

	// delete cluster A
	require.NoError(t, cliA.Delete(context.Background(), &ravendbv1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      keyA.Name,
			Namespace: keyA.Namespace,
		},
	}))

	testutil.WaitObjectDeleted(t, cliA, keyA, timeout)

	testutil.WaitCondition(t, cliB, keyB, ravendbv1.ConditionReady, metav1.ConditionTrue, 5*time.Minute, 2*time.Second)

	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "d"}, &appsv1.StatefulSet{})
	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "e"}, &appsv1.StatefulSet{})
	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.Prefix + "f"}, &appsv1.StatefulSet{})

	assertExists(t, cliB, ctrlclient.ObjectKey{Namespace: nsB, Name: common.RavenDbBootstrapperJob}, &batchv1.Job{})

	assertNotExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "a"}, &appsv1.StatefulSet{})
	assertNotExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "b"}, &appsv1.StatefulSet{})
	assertNotExists(t, cliA, ctrlclient.ObjectKey{Namespace: nsA, Name: common.Prefix + "c"}, &appsv1.StatefulSet{})
}

func assertExists(t *testing.T, cli ctrlclient.Client, key ctrlclient.ObjectKey, obj ctrlclient.Object) {
	t.Helper()
	require.NoError(t, cli.Get(context.Background(), key, obj), "expected %T %s/%s to exist", obj, key.Namespace, key.Name)
}

func assertNotExists(t *testing.T, cli ctrlclient.Client, key ctrlclient.ObjectKey, obj ctrlclient.Object) {
	t.Helper()
	err := cli.Get(context.Background(), key, obj)
	require.Error(t, err, "expected %T %s/%s to NOT exist", obj, key.Namespace, key.Name)
	require.True(t, apierrors.IsNotFound(err), "expected NotFound for %T %s/%s, got: %v", obj, key.Namespace, key.Name, err)
}
