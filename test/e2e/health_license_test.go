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

	corev1 "k8s.io/api/core/v1"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	testutil "ravendb-operator/test/utils"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLicense_L1_Present_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "license-l1-present",
		Namespace: testutil.DefaultNS,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionLicensesValid, metav1.ConditionTrue, timeout, 2*time.Second)

	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1alpha1.ConditionLicensesValid)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1alpha1.ReasonCompleted), cond.Reason)
	require.Contains(t, cond.Message, "license secret present")
}

func TestLicense_L2_DeletedAfterCreate_E2E(t *testing.T) {
	testutil.RecreateTestEnv(t, rbacPath, certHookPath, bootstrapperHookPath)

	cli, key := testutil.CreateCluster(t, testutil.BaseClusterLE, testutil.ClusterCase{
		Name:      "license-l2-deleted",
		Namespace: testutil.DefaultNS,
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionLicensesValid, metav1.ConditionTrue, timeout, 2*time.Second)

	require.NoError(t, cli.Delete(context.Background(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ravendb-license", Namespace: key.Namespace}}))

	testutil.WaitCondition(t, cli, key, ravendbv1alpha1.ConditionLicensesValid, metav1.ConditionFalse, timeout, 2*time.Second)

	cur := &ravendbv1alpha1.RavenDBCluster{}
	require.NoError(t, cli.Get(context.Background(), key, cur))
	cond, ok := testutil.GetCondition(cur, ravendbv1alpha1.ConditionLicensesValid)
	require.True(t, ok)
	require.Equal(t, string(ravendbv1alpha1.ReasonLicenseSecretMissing), cond.Reason)
	require.Contains(t, cond.Message, "missing license secret:")

}
