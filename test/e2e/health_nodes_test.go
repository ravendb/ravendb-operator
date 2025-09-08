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
	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	testutil "ravendb-operator/test/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodes_N1_AllPodsHealthy_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "nodes-n1-healthy",
		Namespace: testutil.DefaultNS,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionNodesHealthy, metav1.ConditionTrue, timeout, 2*time.Second)

	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1alpha1.ConditionNodesHealthy)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1alpha1.ReasonCompleted), cond.Reason)
}

func TestNodes_N2_PodPending_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "nodes-n2-pending",
		Namespace: testutil.DefaultNS,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	podKey := testutil.ObjectKeyForPod(key.Namespace, "a")
	pod := testutil.WaitForPod(t, cli, podKey.Namespace, podKey.Name, 2*time.Minute)
	pod.Status.Phase = corev1.PodPending
	require.NoError(t, cli.Status().Update(context.Background(), pod))

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionNodesHealthy, metav1.ConditionFalse, timeout, 2*time.Second)

	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, _ := testutil.GetCondition(cur, ravendbv1alpha1.ConditionNodesHealthy)
	require.Equal(t, string(ravendbv1alpha1.ReasonWaitingForPods), cond.Reason)
	require.Contains(t, cond.Message, "pods pending:")

}
