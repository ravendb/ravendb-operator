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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStorage_S1_AllPVCsBound_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "storage-s1-all-pvcs-bound",
		Namespace: testutil.DefaultNS,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionStorageReady, metav1.ConditionTrue, 3*time.Minute, 2*time.Second)

	// happy path
	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1alpha1.ConditionStorageReady)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1alpha1.ReasonCompleted), cond.Reason)
}

func TestStorage_S2_OneOrMorePVCNotBound_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	badSC := "does-not-exist-storageclass"

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "storage-s2-pvc-not-bound",
		Namespace: testutil.DefaultNS,
		Modify: func(spec *ravendbv1alpha1.RavenDBClusterSpec) {
			spec.StorageSpec.Data.StorageClassName = &badSC
		},
	})

	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionStorageReady, metav1.ConditionFalse, timeout, 2*time.Second)

	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1alpha1.ConditionStorageReady)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1alpha1.ReasonPVCNotBound), cond.Reason)
	require.True(t, strings.Contains(cond.Message, "PVCs not bound") || strings.Contains(cond.Message, "waiting for PVCs"))
}

func TestStorage_S3_NoPVCsYet_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "storage-s3-no-pvcs-yet",
		Namespace: testutil.DefaultNS,
	})

	testutil.RegisterClusterCleanup(t, cli, key, 3*time.Minute)

	// tight timeout don't let it enough time to bound the pvc
	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionStorageReady, metav1.ConditionFalse, timeout, 200*time.Millisecond)

	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1alpha1.ConditionStorageReady)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1alpha1.ReasonPVCNotBound), cond.Reason)
	require.Contains(t, cond.Message, "PVCs not bound")
}
