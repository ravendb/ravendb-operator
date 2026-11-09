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

package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ravendbv1 "ravendb-operator/api/v1"
)

func database(name string, policy ravendbv1.DeletionPolicy) *ravendbv1.RavenDBDatabase {
	return &ravendbv1.RavenDBDatabase{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: name, Generation: 1},
		Spec: ravendbv1.RavenDBDatabaseSpec{
			ClusterRef:        ravendbv1.ClusterReference{Name: testClusterName},
			DatabaseName:      "Orders",
			ReplicationFactor: 3,
			DeletionPolicy:    policy,
		},
	}
}

func databaseEnv(t *testing.T, objs ...client.Object) (client.Client, *fakeRavenDB, *RavenDBDatabaseReconciler) {
	t.Helper()
	kc := newFakeClient(t, nil, objs...)
	rdb := newFakeRavenDB()
	return kc, rdb, &RavenDBDatabaseReconciler{Client: kc, Recorder: record.NewFakeRecorder(100), Connect: rdb.connect}
}

func getDatabase(t *testing.T, kc client.Client, db *ravendbv1.RavenDBDatabase) *ravendbv1.RavenDBDatabase {
	t.Helper()
	var got ravendbv1.RavenDBDatabase
	require.NoError(t, kc.Get(context.Background(), client.ObjectKeyFromObject(db), &got))
	return &got
}

func TestDatabase_CreatedOnceThenReady(t *testing.T) {
	db := database("orders", ravendbv1.DeletionPolicyRetain)
	kc, rdb, r := databaseEnv(t, cluster(ravendbv1.ModeLetsEncrypt, true), db)

	mustReconcile(t, r, db)
	mustReconcile(t, r, db)

	require.Equal(t, 1, rdb.writes, "reapplying the same spec changes nothing")
	require.Equal(t, map[string]int32{"orders": 3}, rdb.databases)
	got := getDatabase(t, kc, db)
	require.Equal(t, got.Generation, got.Status.ObservedGeneration)
	requireReady(t, got.Status.Conditions)
}

func TestDatabase_WaitsForTheCluster(t *testing.T) {
	for name, tc := range map[string]struct {
		objects []client.Object
		reason  string
	}{
		"missing":          {reason: reasonClusterNotFound},
		"not bootstrapped": {objects: []client.Object{cluster(ravendbv1.ModeLetsEncrypt, false)}, reason: reasonClusterNotReady},
	} {
		t.Run(name, func(t *testing.T) {
			db := database("orders", ravendbv1.DeletionPolicyRetain)
			kc, rdb, r := databaseEnv(t, append(tc.objects, db)...)

			res := mustReconcile(t, r, db)

			require.Zero(t, rdb.writes)
			requireTrue(t, getDatabase(t, kc, db).Status.Conditions, conditionReconciling, tc.reason)
			require.Equal(t, retryInterval, res.RequeueAfter)
		})
	}
}

func TestDatabase_RejectedByRavenDBIsStalled(t *testing.T) {
	db := database("orders", ravendbv1.DeletionPolicyRetain)
	kc, rdb, r := databaseEnv(t, cluster(ravendbv1.ModeLetsEncrypt, true), db)
	rdb.rejectNext = true

	mustReconcile(t, r, db)

	requireTrue(t, getDatabase(t, kc, db).Status.Conditions, conditionStalled, reasonRejected)
}

func TestDatabase_SharedDatabaseIsDroppedWithTheLastDeclaration(t *testing.T) {
	first := database("orders", ravendbv1.DeletionPolicyDelete)
	second := database("orders-copy", ravendbv1.DeletionPolicyDelete)
	second.Spec.DatabaseName = "orders" // RavenDB database names are case-insensitive
	kc, rdb, r := databaseEnv(t, cluster(ravendbv1.ModeLetsEncrypt, true), first, second)
	mustReconcile(t, r, first)
	mustReconcile(t, r, second)
	requireReady(t, getDatabase(t, kc, second).Status.Conditions)

	require.NoError(t, kc.Delete(context.Background(), getDatabase(t, kc, first)))
	mustReconcile(t, r, first)
	require.Equal(t, map[string]int32{"orders": 3}, rdb.databases, "the other resource still declares it")

	require.NoError(t, kc.Delete(context.Background(), getDatabase(t, kc, second)))
	mustReconcile(t, r, second)
	require.Empty(t, rdb.databases)
}

func TestDatabase_DeletionPolicy(t *testing.T) {
	for policy, hardDeleted := range map[ravendbv1.DeletionPolicy][]string{
		ravendbv1.DeletionPolicyRetain: nil,
		ravendbv1.DeletionPolicyDelete: {"Orders"},
	} {
		t.Run(string(policy), func(t *testing.T) {
			db := database("orders", policy)
			kc, rdb, r := databaseEnv(t, cluster(ravendbv1.ModeLetsEncrypt, true), db)
			mustReconcile(t, r, db)

			require.NoError(t, kc.Delete(context.Background(), getDatabase(t, kc, db)))
			mustReconcile(t, r, db)

			require.Equal(t, hardDeleted, rdb.hardDeleted)
			requireGone(t, kc, db)
		})
	}
}

func TestDatabase_DeletionAfterTheClusterIsGone(t *testing.T) {
	db := database("orders", ravendbv1.DeletionPolicyDelete)
	db.Finalizers = []string{cleanupFinalizer}
	kc, rdb, r := databaseEnv(t, db)

	require.NoError(t, kc.Delete(context.Background(), db))
	mustReconcile(t, r, db)

	require.Zero(t, rdb.writes)
	requireGone(t, kc, db)
}
