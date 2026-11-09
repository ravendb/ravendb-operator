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

package v1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// requireNotYetObserved checks what kstatus, and so `helm --wait`, sees right after creation: an observedGeneration
// behind the generation means "in progress" until the operator reports on the resource.
func requireNotYetObserved(t *testing.T, obj client.Object, kind string) {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(GroupVersion.WithKind(kind))
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), u))

	observed, found, err := unstructured.NestedInt64(u.Object, "status", "observedGeneration")
	require.NoError(t, err)
	require.True(t, found, "status.observedGeneration must be present on a new %s", kind)
	require.Less(t, observed, u.GetGeneration())
}

func requireRejectedUpdate(t *testing.T, obj client.Object, message string) {
	t.Helper()
	err := k8sClient.Update(ctx, obj)
	require.Error(t, err)
	require.Contains(t, err.Error(), message)
}

func TestRavenDBDatabase_DefaultsAndImmutableFields(t *testing.T) {
	db := &RavenDBDatabase{
		ObjectMeta: metav1.ObjectMeta{Name: "crd-orders", Namespace: "default"},
		Spec: RavenDBDatabaseSpec{
			ClusterRef:   ClusterReference{Name: "main"},
			DatabaseName: "Orders",
		},
	}
	require.NoError(t, k8sClient.Create(ctx, db))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, db) })

	require.Equal(t, int32(1), db.Spec.ReplicationFactor)
	require.Equal(t, DeletionPolicyRetain, db.Spec.DeletionPolicy)
	requireNotYetObserved(t, db, "RavenDBDatabase")

	for message, mutate := range map[string]func(*RavenDBDatabase){
		"databaseName is immutable":      func(d *RavenDBDatabase) { d.Spec.DatabaseName = "Other" },
		"replicationFactor is immutable": func(d *RavenDBDatabase) { d.Spec.ReplicationFactor = 3 },
		"clusterRef is immutable":        func(d *RavenDBDatabase) { d.Spec.ClusterRef.Name = "other" },
	} {
		changed := db.DeepCopy()
		mutate(changed)
		requireRejectedUpdate(t, changed, message)
	}

	db.Spec.DeletionPolicy = DeletionPolicyDelete
	require.NoError(t, k8sClient.Update(ctx, db), "the deletion policy can change")
}

func TestRavenDBDatabase_RejectsInvalidSpec(t *testing.T) {
	for name, spec := range map[string]RavenDBDatabaseSpec{
		"empty database name":     {ClusterRef: ClusterReference{Name: "main"}},
		"no replicas":             {ClusterRef: ClusterReference{Name: "main"}, DatabaseName: "Orders", ReplicationFactor: -1},
		"unknown policy":          {ClusterRef: ClusterReference{Name: "main"}, DatabaseName: "Orders", DeletionPolicy: "Orphan"},
		"empty cluster reference": {DatabaseName: "Orders"},
	} {
		t.Run(name, func(t *testing.T) {
			db := &RavenDBDatabase{ObjectMeta: metav1.ObjectMeta{Name: "crd-invalid", Namespace: "default"}, Spec: spec}
			require.Error(t, k8sClient.Create(ctx, db))
		})
	}
}

func TestRavenDBClientCertificate_DefaultsAndValidation(t *testing.T) {
	cc := &RavenDBClientCertificate{
		ObjectMeta: metav1.ObjectMeta{Name: "crd-orders-api", Namespace: "default"},
		Spec: RavenDBClientCertificateSpec{
			ClusterRef:  ClusterReference{Name: "main"},
			SecretName:  "orders-api",
			Permissions: map[string]DatabaseAccess{"Orders": DatabaseAccessReadWrite},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, cc))
	t.Cleanup(func() { _ = k8sClient.Delete(ctx, cc) })

	require.Equal(t, ClearanceValidUser, cc.Spec.Clearance)
	require.Equal(t, 8760*time.Hour, cc.Spec.Validity.Duration)
	requireNotYetObserved(t, cc, "RavenDBClientCertificate")

	changed := cc.DeepCopy()
	changed.Spec.SecretName = "other"
	requireRejectedUpdate(t, changed, "secretName is immutable")

	cc.Spec.Permissions = map[string]DatabaseAccess{"Orders": DatabaseAccessRead, "Catalog": DatabaseAccessAdmin}
	cc.Spec.Clearance = ClearanceOperator
	require.NoError(t, k8sClient.Update(ctx, cc), "access can change in place")

	for name, mutate := range map[string]func(*RavenDBClientCertificateSpec){
		"unknown access":      func(s *RavenDBClientCertificateSpec) { s.Permissions = map[string]DatabaseAccess{"Orders": "Write"} },
		"unknown clearance":   func(s *RavenDBClientCertificateSpec) { s.Clearance = "Root" },
		"validity under 1h":   func(s *RavenDBClientCertificateSpec) { s.Validity = metav1.Duration{Duration: 30 * time.Minute} },
		"missing secret name": func(s *RavenDBClientCertificateSpec) { s.SecretName = "" },
		"invalid secret name": func(s *RavenDBClientCertificateSpec) { s.SecretName = "orders_api" },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := &RavenDBClientCertificate{
				ObjectMeta: metav1.ObjectMeta{Name: "crd-invalid", Namespace: "default"},
				Spec:       RavenDBClientCertificateSpec{ClusterRef: ClusterReference{Name: "main"}, SecretName: "s"},
			}
			mutate(&invalid.Spec)
			require.Error(t, k8sClient.Create(ctx, invalid))
		})
	}
}
