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
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/ravendb"
)

const (
	testNamespace   = "shop"
	testClusterName = "main"
	testCASecret    = "ravendb-ca-cert"
	testCAPEM       = "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"
)

// fakeRavenDB keeps databases and certificate registrations in memory and counts the changes made to them.
type fakeRavenDB struct {
	databases   map[string]int32
	hardDeleted []string
	certs       map[string]ravendb.Certificate
	writes      int
	rejectNext  bool
}

func newFakeRavenDB() *fakeRavenDB {
	return &fakeRavenDB{databases: map[string]int32{}, certs: map[string]ravendb.Certificate{}}
}

func (f *fakeRavenDB) connect(context.Context, client.Client, *ravendbv1.RavenDBCluster) (ravenDB, error) {
	return f, nil
}

func (f *fakeRavenDB) write() error {
	f.writes++
	if f.rejectNext {
		f.rejectNext = false
		return &ravendb.APIError{StatusCode: http.StatusBadRequest, Message: "rejected by the fake"}
	}
	return nil
}

func (f *fakeRavenDB) DatabaseExists(_ context.Context, name string) (bool, error) {
	_, ok := f.databases[strings.ToLower(name)]
	return ok, nil
}

func (f *fakeRavenDB) CreateDatabase(_ context.Context, name string, replicationFactor int32) error {
	if err := f.write(); err != nil {
		return err
	}
	f.databases[strings.ToLower(name)] = replicationFactor
	return nil
}

func (f *fakeRavenDB) DeleteDatabase(_ context.Context, name string) error {
	if err := f.write(); err != nil {
		return err
	}
	delete(f.databases, strings.ToLower(name))
	f.hardDeleted = append(f.hardDeleted, name)
	return nil
}

func (f *fakeRavenDB) Certificates(context.Context) ([]ravendb.Certificate, error) {
	out := make([]ravendb.Certificate, 0, len(f.certs))
	for _, cert := range f.certs {
		out = append(out, cert)
	}
	return out, nil
}

func (f *fakeRavenDB) RegisterCertificate(_ context.Context, cert ravendb.Certificate) error {
	if err := f.write(); err != nil {
		return err
	}
	der, err := base64.StdEncoding.DecodeString(cert.Certificate)
	if err != nil {
		return err
	}
	sum := sha1.Sum(der)
	cert.Thumbprint, cert.Certificate = strings.ToUpper(hex.EncodeToString(sum[:])), ""
	f.certs[cert.Thumbprint] = cert
	return nil
}

func (f *fakeRavenDB) EditCertificate(_ context.Context, cert ravendb.Certificate) error {
	if err := f.write(); err != nil {
		return err
	}
	f.certs[cert.Thumbprint] = cert
	return nil
}

func (f *fakeRavenDB) RevokeCertificate(_ context.Context, thumbprint string) error {
	if err := f.write(); err != nil {
		return err
	}
	delete(f.certs, thumbprint)
	return nil
}

func (f *fakeRavenDB) thumbprints() []string {
	out := make([]string, 0, len(f.certs))
	for thumbprint := range f.certs {
		out = append(out, thumbprint)
	}
	sort.Strings(out)
	return out
}

func newFakeClient(t *testing.T, funcs *interceptor.Funcs, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, ravendbv1.AddToScheme(scheme))

	builder := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&ravendbv1.RavenDBDatabase{}, &ravendbv1.RavenDBClientCertificate{})
	if funcs != nil {
		builder = builder.WithInterceptorFuncs(*funcs)
	}
	return builder.Build()
}

func cluster(mode ravendbv1.ClusterMode, bootstrapped bool) *ravendbv1.RavenDBCluster {
	c := &ravendbv1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testClusterName},
		Spec:       ravendbv1.RavenDBClusterSpec{Mode: mode},
	}
	if mode == ravendbv1.ModeNone {
		ca := testCASecret
		c.Spec.CACertSecretRef = &ca
	}
	if bootstrapped {
		c.SetBootstrapped(metav1.Now())
	}
	return c
}

func caSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Name: testCASecret},
		Data:       map[string][]byte{"ca.crt": []byte(testCAPEM)},
	}
}

func reconcileOnce(t *testing.T, r reconcile.Reconciler, obj client.Object) (ctrl.Result, error) {
	t.Helper()
	return r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(obj)})
}

func mustReconcile(t *testing.T, r reconcile.Reconciler, obj client.Object) ctrl.Result {
	t.Helper()
	res, err := reconcileOnce(t, r, obj)
	require.NoError(t, err)
	return res
}

func requireTrue(t *testing.T, conditions []metav1.Condition, conditionType, reason string) {
	t.Helper()
	c := meta.FindStatusCondition(conditions, conditionType)
	require.NotNil(t, c, "condition %s missing in %+v", conditionType, conditions)
	require.Equal(t, metav1.ConditionTrue, c.Status, "condition %s: %s", conditionType, c.Message)
	require.Equal(t, reason, c.Reason, "condition %s: %s", conditionType, c.Message)
}

// requireReady checks the kstatus contract for a reconciled resource: Ready and nothing in progress or stalled.
func requireReady(t *testing.T, conditions []metav1.Condition) {
	t.Helper()
	requireTrue(t, conditions, conditionReady, reasonReconciled)
	require.Nil(t, meta.FindStatusCondition(conditions, conditionReconciling))
	require.Nil(t, meta.FindStatusCondition(conditions, conditionStalled))
}

func requireGone(t *testing.T, kc client.Client, obj client.Object) {
	t.Helper()
	err := kc.Get(context.Background(), client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
	require.True(t, kerrors.IsNotFound(err), "%s still exists: %v", obj.GetName(), err)
}
