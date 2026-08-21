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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"
	testutil "ravendb-operator/test/utils"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	migrationLabNamespace        = "lab"
	migrationLabClusterName      = "pod-template-migration"
	migrationLabImageV1          = "ravendb/ravendb:6.2.11-ubuntu.22.04-x64"
	migrationLabImageV2          = "ravendb/ravendb:7.1.3-ubuntu.22.04-x64"
	migrationLabCertificateImage = "ravendb/operator-upgrade-lab-certificates:local"
)

func TestLab_PodTemplateMigrationOnRealKind(t *testing.T) {
	if os.Getenv("RAVEN_RUN_MIGRATION_LAB") != "1" {
		t.Skip("set RAVEN_RUN_MIGRATION_LAB=1; see labs/01-safe-pod-template-migration.md")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Minute)
	t.Cleanup(cancel)

	artifacts := prepareMigrationLabArtifacts(t, ctx)
	kc := testutil.K8sClient(t)
	createMigrationLabNamespace(t, ctx, kc)
	createMigrationLabSecrets(t, ctx, kc, artifacts)
	createMigrationLabAliasServices(t, ctx, kc)

	cluster := createMigrationLabCluster(t, ctx, kc)
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cleanupCancel()
		_ = kc.Delete(cleanupCtx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: migrationLabNamespace}})
	})
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		diagnosticCtx, diagnosticCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer diagnosticCancel()
		if out, err := testutil.RunKubectl(diagnosticCtx,
			"-n", migrationLabNamespace, "get", "ravendbcluster", migrationLabClusterName, "-o", "yaml",
		); err == nil {
			t.Logf("RavenDBCluster at failure:\n%s", out)
		}
		if out, err := testutil.RunKubectl(diagnosticCtx,
			"-n", migrationLabNamespace, "get", "events", "--sort-by=.lastTimestamp",
		); err == nil {
			t.Logf("lab events at failure:\n%s", out)
		}
		if out, err := testutil.RunKubectl(diagnosticCtx,
			"-n", operatorNS, "logs", "deployment/"+ctlMgrName, "-c", "manager", "--tail=200",
		); err == nil {
			t.Logf("operator log tail at failure:\n%s", out)
		}
	})

	t.Log("phase 1: bootstrap a real RavenDB 6.2.11 cluster through the operator")
	testutil.WaitCondition(t, kc, key, ravendbv1.ConditionReady, metav1.ConditionTrue, 4*time.Minute, time.Second)
	requireBootstrapJobTemplateImmutable(t, ctx, kc)

	t.Log("phase 2: model a healthy cluster created by the pre-revision operator")
	legacyUIDs := make(map[string]string, 3)
	for _, tag := range []string{"A", "B", "C"} {
		legacyUIDs[tag] = makeStatefulSetLegacyAndWait(t, ctx, kc, tag)
	}
	require.Never(t, func() bool {
		for tag, uid := range legacyUIDs {
			if currentPodUID(t, ctx, kc, tag) != uid {
				return true
			}
			requireLegacyTemplate(t, ctx, kc, tag)
		}
		return false
	}, 8*time.Second, 500*time.Millisecond,
		"a healthy cluster must not restart solely because its PodTemplate revision is legacy")

	t.Log("phase 3: an intentional image upgrade carries the current template revision A -> B -> C")
	updateMigrationLabCluster(t, ctx, kc, key, migrationLabImageV2)

	waitStatefulSetImage(t, ctx, kc, "A", migrationLabImageV2)
	requireStatefulSetImage(t, ctx, kc, "B", migrationLabImageV1)
	requireStatefulSetImage(t, ctx, kc, "C", migrationLabImageV1)

	waitStatefulSetImage(t, ctx, kc, "B", migrationLabImageV2)
	requireStatefulSetImage(t, ctx, kc, "C", migrationLabImageV1)

	waitStatefulSetImage(t, ctx, kc, "C", migrationLabImageV2)
	testutil.WaitCondition(t, kc, key, ravendbv1.ConditionReady, metav1.ConditionTrue, 4*time.Minute, time.Second)
	waitNoRolloutMarkers(t, ctx, kc)

	for _, tag := range []string{"A", "B", "C"} {
		waitPodReadyWithImage(t, ctx, kc, tag, migrationLabImageV2)
		require.NotEqual(t, legacyUIDs[tag], currentPodUID(t, ctx, kc, tag))
		requireHardenedTemplate(t, ctx, kc, tag)
		requireRuntimeIdentityAndWritablePVC(t, ctx, tag)
		requireRavenDBVersion(t, ctx, tag, "7.1.3")
	}
}

type migrationLabArtifacts struct {
	caCRT     []byte
	clientPFX []byte
	license   []byte
	serverPFX []byte
}

func prepareMigrationLabArtifacts(t *testing.T, ctx context.Context) migrationLabArtifacts {
	t.Helper()
	licensePath := os.Getenv("RAVEN_LAB_LICENSE_PATH")
	require.NotEmpty(t, licensePath, "set RAVEN_LAB_LICENSE_PATH to a local RavenDB license JSON file")
	license, err := os.ReadFile(licensePath)
	require.NoError(t, err, "read RavenDB license without printing its contents")
	require.NotEmpty(t, license)

	certificateDir := testutil.PathFromRoot(filepath.Join("labs", "upgrade-certificates"))
	require.NoError(t, testutil.RunDocker(ctx, "build", "-t", migrationLabCertificateImage, certificateDir))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = testutil.RunDocker(cleanupCtx, "image", "rm", migrationLabCertificateImage)
	})
	require.NoError(t, testutil.RunDocker(ctx, "pull", migrationLabImageV1))
	require.NoError(t, testutil.RunDocker(ctx, "pull", migrationLabImageV2))
	require.NoError(t, testutil.RunKind(ctx, "load", "docker-image",
		migrationLabImageV1, migrationLabImageV2, "--name", clusterName))

	containerName := fmt.Sprintf("ravendb-upgrade-lab-certs-%d", time.Now().UnixNano())
	// The scratch image is never started. Docker still requires a command in
	// the container config before it will let us copy the generated files out.
	require.NoError(t, testutil.RunDocker(ctx, "create", "--name", containerName,
		migrationLabCertificateImage, "/not-executed"))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		_ = testutil.RunDocker(cleanupCtx, "rm", "-f", containerName)
	})

	certDir := t.TempDir()
	require.NoError(t, testutil.RunDocker(ctx, "cp", containerName+":/lab-certs/.", certDir))

	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(certDir, name))
		require.NoError(t, err)
		return data
	}
	return migrationLabArtifacts{
		caCRT:     read("ca.crt"),
		clientPFX: read("client.pfx"),
		license:   license,
		serverPFX: read("server.pfx"),
	}
}

func createMigrationLabNamespace(t *testing.T, ctx context.Context, kc client.Client) {
	t.Helper()
	err := kc.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: migrationLabNamespace}})
	require.NoError(t, client.IgnoreAlreadyExists(err))
}

func createMigrationLabSecrets(t *testing.T, ctx context.Context, kc client.Client, artifacts migrationLabArtifacts) {
	t.Helper()
	secrets := []*corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "lab-license", Namespace: migrationLabNamespace},
			Data:       map[string][]byte{"license.json": artifacts.license},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "lab-client-cert", Namespace: migrationLabNamespace},
			Data:       map[string][]byte{"client.pfx": artifacts.clientPFX},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "lab-server-cert", Namespace: migrationLabNamespace},
			Data:       map[string][]byte{"server.pfx": artifacts.serverPFX},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "lab-ca-cert", Namespace: migrationLabNamespace},
			Data:       map[string][]byte{"ca.crt": artifacts.caCRT},
		},
	}
	for _, secret := range secrets {
		require.NoError(t, kc.Create(ctx, secret))
	}
}

func createMigrationLabAliasServices(t *testing.T, ctx context.Context, kc client.Client) {
	t.Helper()
	for _, tag := range []string{"A", "B", "C"} {
		lower := strings.ToLower(tag)
		services := []*corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{Name: lower, Namespace: migrationLabNamespace},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{common.LabelNodeTag: tag},
					Ports: []corev1.ServicePort{{
						Name:       "https",
						Port:       443,
						TargetPort: intstr.FromInt(443),
					}},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: lower + "-tcp", Namespace: migrationLabNamespace},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{common.LabelNodeTag: tag},
					Ports: []corev1.ServicePort{{
						Name:       "tcp",
						Port:       38888,
						TargetPort: intstr.FromInt(38888),
					}},
				},
			},
		}
		for _, service := range services {
			require.NoError(t, kc.Create(ctx, service))
		}
	}
}

func createMigrationLabCluster(t *testing.T, ctx context.Context, kc client.Client) *ravendbv1.RavenDBCluster {
	t.Helper()
	cluster := migrationLabCluster()
	deadline := time.NewTimer(45 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		candidate := cluster.DeepCopy()
		lastErr = kc.Create(ctx, candidate)
		if lastErr == nil || kerrors.IsAlreadyExists(lastErr) {
			return cluster
		}
		if !kerrors.IsInternalError(lastErr) &&
			!kerrors.IsServiceUnavailable(lastErr) &&
			!kerrors.IsTimeout(lastErr) &&
			!kerrors.IsServerTimeout(lastErr) &&
			!kerrors.IsTooManyRequests(lastErr) {
			require.NoError(t, lastErr)
		}

		select {
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		case <-deadline.C:
			t.Fatalf("create migration-lab RavenDBCluster after webhook startup: %v", lastErr)
		case <-ticker.C:
		}
	}
}

func migrationLabCluster() *ravendbv1.RavenDBCluster {
	clusterCert := "lab-server-cert"
	caCert := "lab-ca-cert"
	storageClass := "local-path"

	nodes := make([]ravendbv1.RavenDBNode, 0, 3)
	for _, tag := range []string{"A", "B", "C"} {
		lower := strings.ToLower(tag)
		nodes = append(nodes, ravendbv1.RavenDBNode{
			Tag:                tag,
			PublicServerUrl:    fmt.Sprintf("https://%s.lab.svc.cluster.local:443", lower),
			PublicServerUrlTcp: fmt.Sprintf("tcp://%s-tcp.lab.svc.cluster.local:38888", lower),
		})
	}

	return &ravendbv1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{Name: migrationLabClusterName, Namespace: migrationLabNamespace},
		Spec: ravendbv1.RavenDBClusterSpec{
			Image:                migrationLabImageV1,
			ImagePullPolicy:      string(corev1.PullIfNotPresent),
			Mode:                 ravendbv1.ModeNone,
			LicenseSecretRef:     "lab-license",
			ClientCertSecretRef:  "lab-client-cert",
			ClusterCertSecretRef: &clusterCert,
			CACertSecretRef:      &caCert,
			Domain:               "lab.svc.cluster.local",
			Nodes:                nodes,
			StorageSpec: ravendbv1.StorageSpec{
				Data: ravendbv1.VolumeSpec{
					Size:             "10Gi",
					StorageClassName: &storageClass,
				},
			},
		},
	}
}

func requireBootstrapJobTemplateImmutable(t *testing.T, ctx context.Context, kc client.Client) {
	t.Helper()
	var job batchv1.Job
	require.NoError(t, kc.Get(ctx, client.ObjectKey{
		Namespace: migrationLabNamespace,
		Name:      common.RavenDbBootstrapperJob,
	}, &job))
	require.GreaterOrEqual(t, job.Status.Succeeded, int32(1))
	requireHardenedPodTemplate(t, job.Spec.Template, false)

	if job.Spec.Template.Annotations == nil {
		job.Spec.Template.Annotations = map[string]string{}
	}
	job.Spec.Template.Annotations["lab.ravendb.io/immutable-proof"] = "attempted"
	err := kc.Update(ctx, &job)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "immutable")
}

func makeStatefulSetLegacyAndWait(t *testing.T, ctx context.Context, kc client.Client, tag string) string {
	t.Helper()
	sts := getStatefulSet(t, ctx, kc, tag)
	before := currentPodUID(t, ctx, kc, tag)
	require.Equalf(t, common.CurrentPodTemplateRevision,
		sts.Spec.Template.Annotations[common.PodTemplateRevisionAnnotation],
		"node %s was not created with the current revision; template annotations: %#v",
		tag, sts.Spec.Template.Annotations)

	delete(sts.Spec.Template.Annotations, common.PodTemplateRevisionAnnotation)
	sts.Spec.Template.Spec.SecurityContext = nil
	sts.Spec.Template.Spec.Containers[0].SecurityContext = nil
	sts.SetResourceVersion("")
	sts.SetManagedFields(nil)
	sts.Status = appsv1.StatefulSetStatus{}
	sts.TypeMeta = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"}
	require.NoError(t, kc.Patch(ctx, sts, client.Apply,
		client.FieldOwner("ravendb-operator/statefulset"),
		client.ForceOwnership,
	))

	require.Eventually(t, func() bool {
		uid := currentPodUID(t, ctx, kc, tag)
		return uid != "" && uid != before && podReady(t, ctx, kc, tag)
	}, 2*time.Minute, 500*time.Millisecond)
	requireLegacyTemplate(t, ctx, kc, tag)
	return currentPodUID(t, ctx, kc, tag)
}

func requireLegacyTemplate(t *testing.T, ctx context.Context, kc client.Client, tag string) {
	t.Helper()
	sts := getStatefulSet(t, ctx, kc, tag)
	require.Empty(t, sts.Spec.Template.Annotations[common.PodTemplateRevisionAnnotation])

	// The API server may round-trip a removed securityContext as an allocated
	// but empty object. Assert the fields that define this migration instead of
	// relying on pointer nilness.
	if podSecurity := sts.Spec.Template.Spec.SecurityContext; podSecurity != nil {
		require.Nil(t, podSecurity.RunAsNonRoot)
		require.Nil(t, podSecurity.RunAsUser)
		require.Nil(t, podSecurity.RunAsGroup)
		require.Nil(t, podSecurity.FSGroup)
		require.Nil(t, podSecurity.FSGroupChangePolicy)
		require.Nil(t, podSecurity.SeccompProfile)
	}
	if containerSecurity := sts.Spec.Template.Spec.Containers[0].SecurityContext; containerSecurity != nil {
		require.Nil(t, containerSecurity.RunAsNonRoot)
		require.Nil(t, containerSecurity.RunAsUser)
		require.Nil(t, containerSecurity.RunAsGroup)
		require.Nil(t, containerSecurity.AllowPrivilegeEscalation)
		require.Nil(t, containerSecurity.Capabilities)
		require.Nil(t, containerSecurity.SeccompProfile)
	}
}

func requireHardenedTemplate(t *testing.T, ctx context.Context, kc client.Client, tag string) {
	t.Helper()
	sts := getStatefulSet(t, ctx, kc, tag)
	requireHardenedPodTemplate(t, sts.Spec.Template, true)
}

func requireHardenedPodTemplate(t *testing.T, template corev1.PodTemplateSpec, expectLowPortSysctl bool) {
	t.Helper()
	require.Equalf(t, common.CurrentPodTemplateRevision,
		template.Annotations[common.PodTemplateRevisionAnnotation],
		"workload did not receive the current revision; template annotations: %#v",
		template.Annotations)

	podSecurity := template.Spec.SecurityContext
	require.NotNil(t, podSecurity)
	require.Equal(t, int64(common.RavenDBUID), *podSecurity.RunAsUser)
	require.Equal(t, int64(common.RavenDBGID), *podSecurity.RunAsGroup)
	require.Equal(t, int64(common.RavenDBGID), *podSecurity.FSGroup)
	require.True(t, *podSecurity.RunAsNonRoot)
	require.Equal(t, corev1.FSGroupChangeOnRootMismatch, *podSecurity.FSGroupChangePolicy)
	require.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, podSecurity.SeccompProfile.Type)
	if expectLowPortSysctl {
		require.Contains(t, podSecurity.Sysctls, corev1.Sysctl{
			Name: "net.ipv4.ip_unprivileged_port_start", Value: "0",
		})
	} else {
		require.Empty(t, podSecurity.Sysctls)
	}

	require.NotEmpty(t, template.Spec.Containers)
	containerSecurity := template.Spec.Containers[0].SecurityContext
	require.NotNil(t, containerSecurity)
	require.Equal(t, int64(common.RavenDBUID), *containerSecurity.RunAsUser)
	require.Equal(t, int64(common.RavenDBGID), *containerSecurity.RunAsGroup)
	require.True(t, *containerSecurity.RunAsNonRoot)
	require.False(t, *containerSecurity.AllowPrivilegeEscalation)
	require.Contains(t, containerSecurity.Capabilities.Drop, corev1.Capability("ALL"))
	require.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, containerSecurity.SeccompProfile.Type)
}

func requireRuntimeIdentityAndWritablePVC(t *testing.T, ctx context.Context, tag string) {
	t.Helper()
	pod := common.NodeResourceName(tag) + "-0"
	out, err := testutil.ExecInPodCapture(ctx, migrationLabNamespace, pod, "",
		"sh", "-c", `test "$(id -u):$(id -g)" = "999:999" &&
			touch /var/lib/ravendb/data/.upgrade-lab-write-check &&
			rm /var/lib/ravendb/data/.upgrade-lab-write-check &&
			printf '999:999 writable\n'`)
	require.NoError(t, err, "runtime identity/PVC check on %s:\n%s", pod, out)
	require.Contains(t, out, "999:999 writable")
}

func requireRavenDBVersion(t *testing.T, ctx context.Context, tag, version string) {
	t.Helper()
	pod := common.NodeResourceName(tag) + "-0"
	out, err := testutil.ExecInPodCapture(ctx, migrationLabNamespace, pod, "",
		"/usr/lib/ravendb/server/Raven.Server", "--version")
	require.NoError(t, err, "read RavenDB version on %s:\n%s", pod, out)
	require.Contains(t, out, version)
}

func updateMigrationLabCluster(
	t *testing.T,
	ctx context.Context,
	kc client.Client,
	key client.ObjectKey,
	image string,
) {
	t.Helper()
	require.Eventually(t, func() bool {
		var cluster ravendbv1.RavenDBCluster
		if err := kc.Get(ctx, key, &cluster); err != nil {
			return false
		}
		cluster.Spec.Image = image
		return kc.Update(ctx, &cluster) == nil
	}, 30*time.Second, 250*time.Millisecond)
}

func getStatefulSet(t *testing.T, ctx context.Context, kc client.Client, tag string) *appsv1.StatefulSet {
	t.Helper()
	var sts appsv1.StatefulSet
	require.NoError(t, kc.Get(ctx, client.ObjectKey{
		Namespace: migrationLabNamespace,
		Name:      common.NodeResourceName(tag),
	}, &sts))
	return &sts
}

func requireStatefulSetImage(t *testing.T, ctx context.Context, kc client.Client, tag, image string) {
	t.Helper()
	sts := getStatefulSet(t, ctx, kc, tag)
	require.NotEmpty(t, sts.Spec.Template.Spec.Containers)
	require.Equal(t, image, sts.Spec.Template.Spec.Containers[0].Image)
}

func waitStatefulSetImage(t *testing.T, ctx context.Context, kc client.Client, tag, image string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var sts appsv1.StatefulSet
		if err := kc.Get(ctx, client.ObjectKey{
			Namespace: migrationLabNamespace,
			Name:      common.NodeResourceName(tag),
		}, &sts); err != nil {
			return false
		}
		return len(sts.Spec.Template.Spec.Containers) > 0 &&
			sts.Spec.Template.Spec.Containers[0].Image == image
	}, 3*time.Minute, 250*time.Millisecond)
}

func waitPodReadyWithImage(t *testing.T, ctx context.Context, kc client.Client, tag, image string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var pod corev1.Pod
		if err := kc.Get(ctx, client.ObjectKey{
			Namespace: migrationLabNamespace,
			Name:      common.NodeResourceName(tag) + "-0",
		}, &pod); err != nil {
			return false
		}
		return len(pod.Spec.Containers) > 0 &&
			pod.Spec.Containers[0].Image == image &&
			isPodReadyForMigrationLab(&pod)
	}, 3*time.Minute, 500*time.Millisecond)
}

func currentPodUID(t *testing.T, ctx context.Context, kc client.Client, tag string) string {
	t.Helper()
	var pod corev1.Pod
	if err := kc.Get(ctx, client.ObjectKey{
		Namespace: migrationLabNamespace,
		Name:      common.NodeResourceName(tag) + "-0",
	}, &pod); err != nil {
		return ""
	}
	return string(pod.UID)
}

func podReady(t *testing.T, ctx context.Context, kc client.Client, tag string) bool {
	t.Helper()
	var pod corev1.Pod
	if err := kc.Get(ctx, client.ObjectKey{
		Namespace: migrationLabNamespace,
		Name:      common.NodeResourceName(tag) + "-0",
	}, &pod); err != nil {
		return false
	}
	return isPodReadyForMigrationLab(&pod)
}

func isPodReadyForMigrationLab(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func waitNoRolloutMarkers(t *testing.T, ctx context.Context, kc client.Client) {
	t.Helper()
	require.Eventually(t, func() bool {
		for _, tag := range []string{"A", "B", "C"} {
			var sts appsv1.StatefulSet
			if err := kc.Get(ctx, client.ObjectKey{
				Namespace: migrationLabNamespace,
				Name:      common.NodeResourceName(tag),
			}, &sts); err != nil {
				return false
			}
			if _, inFlight := sts.Annotations[common.UpgradeImageAnnotation]; inFlight {
				return false
			}
		}
		return true
	}, 3*time.Minute, 500*time.Millisecond,
		"every rollout transaction must finish before the lab completes")
}
