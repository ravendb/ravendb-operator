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

package resource

import (
	"testing"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func strPtr(s string) *string { return &s }

func minimalCluster() *ravendbv1.RavenDBCluster {
	return &ravendbv1.RavenDBCluster{
		Spec: ravendbv1.RavenDBClusterSpec{
			Image:                "ravendb/ravendb:latest",
			ImagePullPolicy:      "IfNotPresent",
			Mode:                 ravendbv1.ModeNone,
			LicenseSecretRef:     "ravendb-license",
			ClusterCertSecretRef: strPtr("ravendb-server-cert"),
			ClientCertSecretRef:  "ravendb-client-cert",
			Nodes: []ravendbv1.RavenDBNode{
				{Tag: "a", PublicServerUrl: "https://a.example:443", PublicServerUrlTcp: "tcp://a-tcp.example:443"},
			},
			StorageSpec: ravendbv1.StorageSpec{
				Data: ravendbv1.VolumeSpec{Size: "10Gi"},
			},
		},
	}
}

// assertHardenedContainer verifies the PodSecurity "restricted" fields that must
// live on every container.
func assertHardenedContainer(t *testing.T, sc *corev1.SecurityContext) {
	t.Helper()
	require.NotNil(t, sc, "container SecurityContext must be set")
	require.NotNil(t, sc.RunAsUser)
	require.EqualValues(t, common.RavenDBUID, *sc.RunAsUser)
	require.NotNil(t, sc.RunAsGroup)
	require.EqualValues(t, common.RavenDBGID, *sc.RunAsGroup)
	require.NotNil(t, sc.RunAsNonRoot)
	require.True(t, *sc.RunAsNonRoot)
	require.NotNil(t, sc.AllowPrivilegeEscalation)
	require.False(t, *sc.AllowPrivilegeEscalation)
	require.NotNil(t, sc.Capabilities)
	require.Equal(t, []corev1.Capability{"ALL"}, sc.Capabilities.Drop)
	require.NotNil(t, sc.SeccompProfile)
	require.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, sc.SeccompProfile.Type)
}

// assertHardenedPod verifies the shared pod-level context, including the fsGroup
// that fixes the DataDir permission failure on root-owned PVC mounts (#37/#39).
func assertHardenedPod(t *testing.T, sc *corev1.PodSecurityContext) {
	t.Helper()
	require.NotNil(t, sc, "pod SecurityContext must be set")
	require.NotNil(t, sc.FSGroup)
	require.EqualValues(t, common.RavenDBGID, *sc.FSGroup)
	require.NotNil(t, sc.FSGroupChangePolicy)
	require.Equal(t, corev1.FSGroupChangeOnRootMismatch, *sc.FSGroupChangePolicy)
	require.NotNil(t, sc.RunAsNonRoot)
	require.True(t, *sc.RunAsNonRoot)
	require.NotNil(t, sc.SeccompProfile)
	require.Equal(t, corev1.SeccompProfileTypeRuntimeDefault, sc.SeccompProfile.Type)
}

// The StatefulSet pod must carry the hardened pod context AND the low-port
// sysctl, and its RavenDB container must be hardened. Testing through
// BuildStatefulSet (not the helper in isolation) guards the wiring.
func TestStatefulSetHasHardenedSecurityContext(t *testing.T) {
	sts, err := BuildStatefulSet(minimalCluster(), ravendbv1.RavenDBNode{Tag: "a"})
	require.NoError(t, err)

	podSpec := sts.Spec.Template.Spec
	assertHardenedPod(t, podSpec.SecurityContext)
	require.Equal(t, common.CurrentPodTemplateRevision, sts.Spec.Template.Annotations[common.PodTemplateRevisionAnnotation])

	require.Contains(t, podSpec.SecurityContext.Sysctls, corev1.Sysctl{
		Name:  "net.ipv4.ip_unprivileged_port_start",
		Value: "0",
	}, "StatefulSet pod must keep the low-port sysctl to bind 443")

	require.Len(t, podSpec.Containers, 1)
	assertHardenedContainer(t, podSpec.Containers[0].SecurityContext)
}

// The bootstrapper Job pod must be hardened too (else PodSecurity "restricted"
// admission rejects it), but it does not bind low ports, so no sysctl.
func TestJobHasHardenedSecurityContext(t *testing.T) {
	job, err := BuildJob(minimalCluster())
	require.NoError(t, err)

	podSpec := job.Spec.Template.Spec
	assertHardenedPod(t, podSpec.SecurityContext)
	require.Equal(t, common.CurrentPodTemplateRevision, job.Spec.Template.Annotations[common.PodTemplateRevisionAnnotation])
	require.Empty(t, podSpec.SecurityContext.Sysctls, "bootstrapper pod needs no sysctls")

	require.Len(t, podSpec.Containers, 1)
	assertHardenedContainer(t, podSpec.Containers[0].SecurityContext)
}

// The fsGroup must match the container UID/GID: they are the same identity
// (the RavenDB image's USER 999), so a drift between them would reintroduce the
// permission failure.
func TestPodFsGroupMatchesContainerIdentity(t *testing.T) {
	pod := buildPodSecurityContext()
	container := buildContainerSecurityContext()

	require.EqualValues(t, *container.RunAsGroup, *pod.FSGroup)
	require.EqualValues(t, common.RavenDBGID, *pod.FSGroup)
}
