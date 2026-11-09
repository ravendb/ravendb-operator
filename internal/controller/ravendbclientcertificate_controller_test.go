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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/clientcert"
	"ravendb-operator/pkg/ravendb"
)

const testValidity = 8760 * time.Hour

// certificateEnv is one RavenDBClientCertificate against a bootstrapped cluster, with a clock the test moves.
type certificateEnv struct {
	t   *testing.T
	kc  client.Client
	rdb *fakeRavenDB
	r   *RavenDBClientCertificateReconciler
	cc  *ravendbv1.RavenDBClientCertificate
	now time.Time
}

func newCertificateEnv(t *testing.T, mode ravendbv1.ClusterMode, funcs *interceptor.Funcs, extra ...client.Object) *certificateEnv {
	t.Helper()
	cc := &ravendbv1.RavenDBClientCertificate{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "orders-api", UID: "cc-uid", Generation: 1},
		Spec: ravendbv1.RavenDBClientCertificateSpec{
			ClusterRef:  ravendbv1.ClusterReference{Name: testClusterName},
			SecretName:  "orders-api-cert",
			Clearance:   ravendbv1.ClearanceValidUser,
			Permissions: map[string]ravendbv1.DatabaseAccess{"Orders": ravendbv1.DatabaseAccessReadWrite},
			Validity:    metav1.Duration{Duration: testValidity},
		},
	}
	env := &certificateEnv{t: t, rdb: newFakeRavenDB(), cc: cc, now: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)}
	env.kc = newFakeClient(t, funcs, append([]client.Object{cluster(mode, true), caSecret(), cc}, extra...)...)
	env.r = &RavenDBClientCertificateReconciler{
		Client: env.kc, APIReader: env.kc, Scheme: env.kc.Scheme(), Recorder: record.NewFakeRecorder(100),
		Connect: env.rdb.connect, Now: func() time.Time { return env.now },
	}
	return env
}

func (e *certificateEnv) reconcile() time.Duration {
	e.t.Helper()
	return mustReconcile(e.t, e.r, e.cc).RequeueAfter
}

func (e *certificateEnv) resource() *ravendbv1.RavenDBClientCertificate {
	e.t.Helper()
	var got ravendbv1.RavenDBClientCertificate
	require.NoError(e.t, e.kc.Get(context.Background(), client.ObjectKeyFromObject(e.cc), &got))
	return &got
}

func (e *certificateEnv) secret() *corev1.Secret {
	e.t.Helper()
	var secret corev1.Secret
	require.NoError(e.t, e.kc.Get(context.Background(), client.ObjectKey{Namespace: testNamespace, Name: e.cc.Spec.SecretName}, &secret))
	return &secret
}

// stored is the certificate an application mounting the Secret would use.
func (e *certificateEnv) stored() *clientcert.Certificate {
	e.t.Helper()
	cert, err := clientcert.Decode(e.secret().Data[clientPFXKey])
	require.NoError(e.t, err)
	return cert
}

func TestCertificate_IssuedRegisteredAndStored(t *testing.T) {
	env := newCertificateEnv(t, ravendbv1.ModeNone, nil)

	requeueAfter := env.reconcile()
	env.reconcile()

	require.Equal(t, 1, env.rdb.writes, "reapplying the same spec changes nothing")
	cert := env.stored()
	require.Equal(t, ravendb.Certificate{
		Name:              "shop/orders-api",
		Thumbprint:        cert.Thumbprint(),
		SecurityClearance: "ValidUser",
		Permissions:       map[string]string{"Orders": "ReadWrite"},
	}, env.rdb.certs[cert.Thumbprint()])
	require.Equal(t, testCAPEM, string(env.secret().Data[caCertKey]), "a private CA is handed to the application")
	require.True(t, metav1.IsControlledBy(env.secret(), env.cc), "the Secret is garbage-collected with the resource")

	got := env.resource()
	requireReady(t, got.Status.Conditions)
	require.Equal(t, cert.Thumbprint(), got.Status.Thumbprint)
	require.Equal(t, resyncInterval, requeueAfter)
}

func TestCertificate_RenewalKeepsThePreviousCertificateUntilTheNextRenewal(t *testing.T) {
	env := newCertificateEnv(t, ravendbv1.ModeLetsEncrypt, nil)
	env.reconcile()
	first := env.stored()

	env.now = first.RenewAt(testValidity).Add(-time.Minute)
	require.Equal(t, time.Minute, env.reconcile(), "the next reconcile is due at renewal")

	env.now = first.RenewAt(testValidity).Add(time.Minute)
	env.reconcile()
	second := env.stored()
	require.ElementsMatch(t, []string{first.Thumbprint(), second.Thumbprint()}, env.rdb.thumbprints(),
		"applications that have not reloaded the Secret keep working")

	env.now = second.RenewAt(testValidity).Add(time.Minute)
	env.reconcile()
	require.ElementsMatch(t, []string{second.Thumbprint(), env.stored().Thumbprint()}, env.rdb.thumbprints())

	require.NoError(t, env.kc.Delete(context.Background(), env.resource()))
	env.reconcile()
	require.Empty(t, env.rdb.thumbprints(), "deleting the resource revokes every certificate it issued")
	requireGone(t, env.kc, env.cc)
}

func TestCertificate_DeletionWaitsUntilTheClusterCanRevoke(t *testing.T) {
	env := newCertificateEnv(t, ravendbv1.ModeLetsEncrypt, nil)
	env.reconcile()
	setBootstrapped := func(done bool) {
		var current ravendbv1.RavenDBCluster
		require.NoError(t, env.kc.Get(context.Background(), client.ObjectKey{Namespace: testNamespace, Name: testClusterName}, &current))
		current.Status = cluster(ravendbv1.ModeLetsEncrypt, done).Status
		require.NoError(t, env.kc.Update(context.Background(), &current))
	}
	setBootstrapped(false)

	require.NoError(t, env.kc.Delete(context.Background(), env.resource()))
	_, err := reconcileOnce(t, env.r, env.cc)
	require.Error(t, err, "the deletion is retried")
	require.Len(t, env.rdb.thumbprints(), 1, "the certificate is still trusted, so its resource stays")
	env.resource()

	setBootstrapped(true)
	env.reconcile()
	require.Empty(t, env.rdb.thumbprints())
	requireGone(t, env.kc, env.cc)
}

func TestCertificate_RegistrationWithoutASecretIsRevokedByTheNextAttempt(t *testing.T) {
	secretWrites := 0
	failFirstSecretWrite := &interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				if secretWrites++; secretWrites == 1 {
					return errors.New("API server unavailable")
				}
			}
			return c.Create(ctx, obj, opts...)
		},
	}
	env := newCertificateEnv(t, ravendbv1.ModeLetsEncrypt, failFirstSecretWrite)

	env.reconcile()
	require.Len(t, env.rdb.thumbprints(), 1, "registered before the Secret write failed")
	env.reconcile()

	require.Equal(t, []string{env.stored().Thumbprint()}, env.rdb.thumbprints())
	requireReady(t, env.resource().Status.Conditions)
}

func TestCertificate_SecretKubernetesRefusesIsStalled(t *testing.T) {
	quotaExceeded := &interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				return kerrors.NewForbidden(corev1.Resource("secrets"), obj.GetName(), errors.New("exceeded quota"))
			}
			return c.Create(ctx, obj, opts...)
		},
	}
	env := newCertificateEnv(t, ravendbv1.ModeLetsEncrypt, quotaExceeded)

	requeueAfter := env.reconcile()

	requireTrue(t, env.resource().Status.Conditions, conditionStalled, reasonSecretRejected)
	require.Equal(t, resyncInterval, requeueAfter, "not a new certificate every few seconds")
}

func TestCertificate_ForeignSecretIsNeverOverwritten(t *testing.T) {
	foreign := func() *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: "orders-api-cert"},
			Data:       map[string][]byte{"keep": []byte("me")},
		}
	}
	createdMeanwhile := &interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if _, ok := obj.(*corev1.Secret); ok {
				_ = c.Create(ctx, foreign())
			}
			return c.Create(ctx, obj, opts...)
		},
	}

	for name, tc := range map[string]struct {
		funcs    *interceptor.Funcs
		existing []client.Object
	}{
		"created before the resource":            {existing: []client.Object{foreign()}},
		"created between the read and the write": {funcs: createdMeanwhile},
	} {
		t.Run(name, func(t *testing.T) {
			env := newCertificateEnv(t, ravendbv1.ModeLetsEncrypt, tc.funcs, tc.existing...)

			env.reconcile()
			env.reconcile()

			require.Equal(t, foreign().Data, env.secret().Data)
			requireTrue(t, env.resource().Status.Conditions, conditionStalled, reasonSecretConflict)
		})
	}
}
