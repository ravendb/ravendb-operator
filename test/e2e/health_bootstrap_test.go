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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBootstrap_B1_JobSucceeded_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "bootstrap-b1-succeeded",
		Namespace: testutil.DefaultNS,
	})
	testutil.RegisterClusterCleanup(t, cli, key, 3*time.Minute)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionBootstrapCompleted, metav1.ConditionTrue, 5*time.Minute, 2*time.Second)

	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1alpha1.ConditionBootstrapCompleted)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1alpha1.ReasonCompleted), cond.Reason)
	require.Equal(t, ravendbv1alpha1.PhaseRunning, cur.Status.Phase)

}

func TestBootstrap_B2_JobRunning_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "bootstrap-b2-running",
		Namespace: testutil.DefaultNS,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionBootstrapCompleted, metav1.ConditionFalse, timeout, 500*time.Millisecond)

	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1alpha1.ConditionBootstrapCompleted)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1alpha1.ReasonBootstrapJobRunning), cond.Reason)
	require.Equal(t, ravendbv1alpha1.PhaseDeploying, cur.Status.Phase)
}
