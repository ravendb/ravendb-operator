package e2e

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	ravendbv1 "ravendb-operator/api/v1"
	testutil "ravendb-operator/test/utils"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// TestClusterChart_C7 installs a cluster with two databases and two application certificates from one chart and
// checks the lifecycle the operator promises for them.
const c7Namespace = "ravendb-cluster-chart-c7"

func TestClusterChart_C7_DatabasesAndClientCertificates_E2E(t *testing.T) {
	const ns = c7Namespace

	cli, key := testutil.InstallClusterChart(t, testutil.ClusterChartCase{
		Name:             "clusterchart-c7-apps",
		Namespace:        ns,
		IngressClassName: "nginx",
		Provision:        true,
		InstallTimeout:   10 * time.Minute,
		Values: []string{
			"databases[0].name=Orders",
			"databases[0].replicationFactor=3",
			"databases[1].name=Catalog",
			"databases[1].replicationFactor=3", // node A, which the checks ask, must host it: other nodes answer 410
			"clientCertificates[0].name=orders-api",
			"clientCertificates[0].permissions.Orders=ReadWrite",
			"clientCertificates[1].name=catalog-api",
			"clientCertificates[1].permissions.Catalog=ReadWrite",
		},
	})
	testutil.RegisterClusterCleanup(t, cli, key, timeout)
	testutil.WaitCondition(t, cli, key, ravendbv1.ConditionReady, metav1.ConditionTrue, timeout, 2*time.Second)

	orders := &ravendbv1.RavenDBDatabase{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "orders"}}
	catalog := &ravendbv1.RavenDBDatabase{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "catalog"}}
	ordersAPI := &ravendbv1.RavenDBClientCertificate{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "orders-api"}}
	catalogAPI := &ravendbv1.RavenDBClientCertificate{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "catalog-api"}}
	for _, db := range []*ravendbv1.RavenDBDatabase{orders, catalog} {
		waitReconciled(t, cli, db, func() (int64, []metav1.Condition) {
			return db.Status.ObservedGeneration, db.Status.Conditions
		})
	}
	for _, cc := range []*ravendbv1.RavenDBClientCertificate{ordersAPI, catalogAPI} {
		waitReconciled(t, cli, cc, func() (int64, []metav1.Condition) {
			return cc.Status.ObservedGeneration, cc.Status.Conditions
		})
	}

	ordersPFX := appPFX(t, cli, "orders-api")
	catalogPFX := appPFX(t, cli, "catalog-api")

	t.Run("each application reaches only its own database", func(t *testing.T) {
		require.Equal(t, http.StatusOK, appStatusCode(t, ordersPFX, "Orders"))
		require.Equal(t, http.StatusForbidden, appStatusCode(t, ordersPFX, "Catalog"))
		require.Equal(t, http.StatusOK, appStatusCode(t, catalogPFX, "Catalog"))
		require.Equal(t, http.StatusForbidden, appStatusCode(t, catalogPFX, "Orders"))
	})

	t.Run("a permission change keeps the certificate", func(t *testing.T) {
		require.NoError(t, cli.Get(context.Background(), ctrlclient.ObjectKeyFromObject(ordersAPI), ordersAPI))
		ordersAPI.Spec.Permissions["Catalog"] = ravendbv1.DatabaseAccessReadWrite
		require.NoError(t, cli.Update(context.Background(), ordersAPI))
		waitReconciled(t, cli, ordersAPI, func() (int64, []metav1.Condition) {
			return ordersAPI.Status.ObservedGeneration, ordersAPI.Status.Conditions
		})

		require.Equal(t, ordersPFX, appPFX(t, cli, "orders-api"))
		require.Equal(t, http.StatusOK, appStatusCode(t, ordersPFX, "Catalog"))
	})

	t.Run("a lost Secret gets a new certificate and the old one is revoked", func(t *testing.T) {
		lost := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "orders-api"}}
		require.NoError(t, cli.Delete(context.Background(), lost))

		var replacement []byte
		require.Eventually(t, func() bool {
			var secret corev1.Secret
			if err := cli.Get(context.Background(), testutil.Key(ns, "orders-api"), &secret); err != nil {
				return false
			}
			replacement = secret.Data["client.pfx"]
			return len(replacement) > 0 && string(replacement) != string(ordersPFX)
		}, timeout, 2*time.Second, "the Secret was not recreated")

		require.Equal(t, http.StatusOK, appStatusCode(t, replacement, "Orders"))
		require.Equal(t, http.StatusForbidden, appStatusCode(t, ordersPFX, "Orders"), "the old certificate is revoked")
	})

	t.Run("deleting a client certificate revokes it", func(t *testing.T) {
		require.NoError(t, cli.Delete(context.Background(), catalogAPI))
		waitGone(t, cli, catalogAPI)

		require.Equal(t, http.StatusForbidden, appStatusCode(t, catalogPFX, "Catalog"), "the certificate is revoked")
	})

	t.Run("deleting a database with the default policy keeps the data", func(t *testing.T) {
		require.NoError(t, cli.Delete(context.Background(), orders))
		waitGone(t, cli, orders)

		require.Equal(t, http.StatusOK, appStatusCode(t, appPFX(t, cli, "orders-api"), "Orders"))
	})
}

// waitReconciled waits until the operator has observed the latest spec and reports Ready.
func waitReconciled(
	t *testing.T, cli ctrlclient.Client, obj ctrlclient.Object, status func() (int64, []metav1.Condition),
) {
	t.Helper()
	require.Eventually(t, func() bool {
		if err := cli.Get(context.Background(), ctrlclient.ObjectKeyFromObject(obj), obj); err != nil {
			return false
		}
		observed, conditions := status()
		return observed == obj.GetGeneration() && meta.IsStatusConditionTrue(conditions, "Ready")
	}, timeout, 2*time.Second, "%T %s never became Ready", obj, obj.GetName())
}

func waitGone(t *testing.T, cli ctrlclient.Client, obj ctrlclient.Object) {
	t.Helper()
	require.Eventually(t, func() bool {
		return apierrors.IsNotFound(cli.Get(context.Background(), ctrlclient.ObjectKeyFromObject(obj), obj))
	}, timeout, 2*time.Second, "%T %s was not deleted", obj, obj.GetName())
}

func appPFX(t *testing.T, cli ctrlclient.Client, secretName string) []byte {
	t.Helper()
	var secret corev1.Secret
	require.NoError(t, cli.Get(context.Background(), testutil.Key(c7Namespace, secretName), &secret))
	require.NotContains(t, secret.Data, "ca.crt", "Let's Encrypt clusters need no private CA")
	return secret.Data["client.pfx"]
}

// appStatusCode is the HTTP status an application holding the PFX gets for the database, asked from inside node A.
func appStatusCode(t *testing.T, pfx []byte, database string) int {
	t.Helper()
	script := fmt.Sprintf(`set -e
echo %s | base64 -d > /tmp/app.pfx
openssl pkcs12 -in /tmp/app.pfx -passin pass: -nodes -out /tmp/app.pem 2>/dev/null
curl -sS -o /dev/null -w '%%{http_code}' --cert /tmp/app.pem \
  https://a.ravendb-operator-e2e.ravendb.run/databases/%s/stats
rm -f /tmp/app.pfx /tmp/app.pem`, base64.StdEncoding.EncodeToString(pfx), database)

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	out, err := testutil.ExecInPodCapture(ctx, c7Namespace, "ravendb-a-0", "", "sh", "-c", script)
	require.NoError(t, err, out)

	fields := strings.Fields(out) // kubectl may add notices to the output; curl writes the status last
	require.NotEmpty(t, fields, "no curl output")
	code, err := strconv.Atoi(fields[len(fields)-1])
	require.NoError(t, err, "unexpected curl output: %q", out)
	return code
}
