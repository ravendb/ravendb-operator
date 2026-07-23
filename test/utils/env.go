package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	ravendbv1 "ravendb-operator/api/v1"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

type ClusterCase struct {
	Name      string
	Namespace string
	Modify    func(*ravendbv1.RavenDBClusterSpec)
}

const DefaultNS = "ravendb"

func CreateNamespace(ns string) env.Func { return envfuncs.CreateNamespace(ns) }
func DeleteNamespace(ns string) env.Func { return envfuncs.DeleteNamespace(ns) }

func BindKubectlToSuiteEnv() env.Func {
	return func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		if kc := c.KubeconfigFile(); kc != "" {
			_ = os.Setenv("KUBECONFIG", kc)
		}
		return ctx, nil
	}
}

func CreateCluster(t *testing.T, base func(name string) *ravendbv1.RavenDBCluster, tc ClusterCase) (ctrlclient.Client, ctrlclient.ObjectKey) {
	t.Helper()
	cli := K8sClient(t)
	name := SanitizeName("e2e-" + tc.Name)

	obj := base(name)
	if tc.Namespace != "" {
		obj.Namespace = tc.Namespace
	}
	if tc.Modify != nil {
		tc.Modify(&obj.Spec)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	require.NoError(t, cli.Create(ctx, obj))

	k := Key(obj.Namespace, obj.Name)
	WaitReadable(t, cli, k, 90*time.Second)
	return cli, k
}

func EnsureNamespace(t *testing.T, ns string, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cl := K8sClient(t)
	tmp := &corev1.Namespace{}
	err := cl.Get(ctx, ctrlclient.ObjectKey{Name: ns}, tmp)
	if apierrors.IsNotFound(err) {
		require.NoError(t, cl.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}))
		return
	}
	require.NoError(t, err)
}

func EnsureKustomize(t *testing.T, path string, timeout time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := ApplyKustomize(path)(ctx, envconf.New())
	require.NoError(t, err)
}

func RepoRoot() string {
	if r := os.Getenv("PROJECT_ROOT"); r != "" {
		return r
	}
	wd, _ := os.Getwd()
	return wd
}

func PathFromRoot(rel string) string { return filepath.Join(RepoRoot(), rel) }

func WaitReadable(t *testing.T, cli ctrlclient.Client, k ctrlclient.ObjectKey, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		tmp := &ravendbv1.RavenDBCluster{}
		return cli.Get(context.Background(), k, tmp) == nil
	}, timeout, 500*time.Millisecond)
}

func WaitCondition(t *testing.T, cli ctrlclient.Client, k ctrlclient.ObjectKey, condType ravendbv1.ClusterConditionType, want metav1.ConditionStatus, timeout, interval time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for {
		cur := &ravendbv1.RavenDBCluster{}
		if err := cli.Get(context.Background(), k, cur); err == nil {
			if cond, ok := GetCondition(cur, condType); ok && cond.Status == want {
				return
			}
		}
		if time.Now().After(deadline) {
			// Dump cluster state while the kind cluster is still alive, so CI
			// logs show *why* readiness stalled instead of a bare timeout.
			DumpClusterDiagnostics(t, k.Namespace)
			t.Fatalf("condition %s did not become %s within %s", condType, want, timeout)
			return
		}
		time.Sleep(interval)
	}
}

// DumpClusterDiagnostics prints best-effort cluster state (the RavenDBCluster
// status conditions, pods across namespaces, events, and operator logs) to the
// test log. It is invoked when a wait times out, while the kind cluster is
// still alive, so a CI failure shows which reconcile stage stalled instead of a
// bare "condition never satisfied".
func DumpClusterDiagnostics(t *testing.T, ns string) {
	t.Helper()
	ctx := context.Background()
	show := func(title string, args ...string) {
		out, err := RunKubectlCapture(ctx, args...)
		if err != nil {
			out += fmt.Sprintf("\n(kubectl error: %v)", err)
		}
		t.Logf("\n========== DIAGNOSTICS: %s ==========\n%s", title, out)
	}
	show("RavenDBCluster/"+ns, "-n", ns, "get", "ravendbcluster", "-o", "yaml")
	show("pods (all namespaces)", "get", "pods", "-A", "-o", "wide")
	show("events/"+ns, "-n", ns, "get", "events", "--sort-by=.lastTimestamp")
	show("describe pods/"+ns, "-n", ns, "describe", "pods")
	show("ravendb node logs/"+ns, "-n", ns, "logs", "-l", "app.kubernetes.io/name=ravendb", "--all-containers", "--tail=300", "--prefix")
	show("ravendb node logs (previous)/"+ns, "-n", ns, "logs", "-l", "app.kubernetes.io/name=ravendb", "--all-containers", "--previous", "--tail=300", "--prefix")
	show("bootstrap job logs/"+ns, "-n", ns, "logs", "-l", "job-name=ravendb-cluster-init", "--all-containers", "--tail=400", "--prefix")
	show("bootstrap job logs (previous)/"+ns, "-n", ns, "logs", "-l", "job-name=ravendb-cluster-init", "--all-containers", "--previous", "--tail=400", "--prefix")
	show("operator logs", "-n", "ravendb-operator-system", "logs", "-l", "control-plane=controller-manager", "--tail=300", "--all-containers", "--prefix")
}

func GetCondition(obj *ravendbv1.RavenDBCluster, t ravendbv1.ClusterConditionType) (metav1.Condition, bool) {
	for i := range obj.Status.Conditions {
		c := obj.Status.Conditions[i]
		if c.Type == string(t) {
			return c, true
		}
	}
	return metav1.Condition{}, false
}

func RegisterClusterCleanup(t *testing.T, cli ctrlclient.Client, key ctrlclient.ObjectKey, timeout time.Duration) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		nsName := key.Namespace

		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
		_ = cli.Delete(ctx, ns)

		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			cur := &corev1.Namespace{}
			err := cli.Get(ctx, ctrlclient.ObjectKey{Name: nsName}, cur)
			if apierrors.IsNotFound(err) {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}

		newNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
		if err := cli.Create(ctx, newNS); err != nil && !apierrors.IsAlreadyExists(err) {
			t.Logf("namespace recreate failed: %v", err)
			return
		}

		saDeadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(saDeadline) {
			sa := &corev1.ServiceAccount{}
			err := cli.Get(ctx, ctrlclient.ObjectKey{Namespace: nsName, Name: "default"}, sa)
			if err == nil {
				return
			}
			if !apierrors.IsNotFound(err) {
				t.Logf("waiting for default SA: %v", err)
			}
			time.Sleep(300 * time.Millisecond)
		}
	})
}

func RecreateTestEnv(t *testing.T) {
	t.Helper()

	EnsureNamespace(t, DefaultNS, 60*time.Second)

	SeedSecrets(t)

}

func RecreateTestEnvInNamespace(t *testing.T, ns string) {
	t.Helper()

	EnsureNamespace(t, ns, 60*time.Second)
	SeedLESecretsInNamespace(t, ns, 2*time.Minute)
}

func ObjectKeyForPod(ns, tag string) ctrlclient.ObjectKey {
	return ctrlclient.ObjectKey{
		Namespace: ns,
		Name:      "ravendb-" + tag + "-0",
	}
}

func WaitForPod(t *testing.T, cli ctrlclient.Client, ns, name string, timeout time.Duration) *corev1.Pod {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	pod := &corev1.Pod{}
	require.Eventually(t, func() bool {
		return cli.Get(ctx, ctrlclient.ObjectKey{Namespace: ns, Name: name}, pod) == nil
	}, timeout, 500*time.Millisecond, "pod %s/%s did not appear", ns, name)

	return pod
}

func PatchSpecImage(t *testing.T, cli ctrlclient.Client, key ctrlclient.ObjectKey, img string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	cur := &ravendbv1.RavenDBCluster{}
	require.NoError(t, cli.Get(ctx, key, cur))
	cur.Spec.Image = img
	require.NoError(t, cli.Update(ctx, cur))
}

func WaitPodImage(t *testing.T, cli ctrlclient.Client, ns, podName, want string, timeout time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		p := WaitForPod(t, cli, ns, podName, 45*time.Second)
		if len(p.Spec.Containers) == 0 {
			return false
		}
		return p.Spec.Containers[0].Image == want
	}, timeout, 2*time.Second, "pod %s did not switch image to %s", podName, want)
}

func LogStart(t *testing.T) {
	t.Helper()
	t.Logf("START: %s", t.Name())
}

func WaitObjectDeleted(t *testing.T, cli client.Client, key client.ObjectKey, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for {
		obj := &ravendbv1.RavenDBCluster{}
		err := cli.Get(context.Background(), key, obj)

		if apierrors.IsNotFound(err) {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for object %s/%s to be deleted", key.Namespace, key.Name)
		}

		time.Sleep(1 * time.Second)
	}
}
