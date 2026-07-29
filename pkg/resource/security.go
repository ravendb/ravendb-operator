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
	"ravendb-operator/pkg/common"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func buildPodTemplateObjectMeta(labels map[string]string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Labels: labels,
		Annotations: map[string]string{
			common.PodTemplateRevisionAnnotation: common.CurrentPodTemplateRevision,
		},
	}
}

// buildContainerSecurityContext returns the hardened, PodSecurity "restricted"
// compatible container context shared by every RavenDB container (main node and
// bootstrapper). Both run the same image, so both carry the same identity.
//
// allowPrivilegeEscalation and capabilities are set per-container because the
// "restricted" profile requires them there (there is no pod-level equivalent).
// Dropping ALL capabilities is safe even though the node binds port 443: the
// pod sysctl net.ipv4.ip_unprivileged_port_start=0 makes 443 an unprivileged
// port, so NET_BIND_SERVICE is not needed.
func buildContainerSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot:             pointer.Bool(true),
		RunAsUser:                pointer.Int64(common.RavenDBUID),
		RunAsGroup:               pointer.Int64(common.RavenDBGID),
		AllowPrivilegeEscalation: pointer.Bool(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
}

// buildPodSecurityContext returns the shared pod-level context. FSGroup matches
// the container GID so Kubernetes prepares mounted volumes (PVCs) for the
// non-root process: a freshly provisioned PVC is often mounted root-owned, and
// without fsGroup uid/gid 999 cannot write to its own DataDir. OnRootMismatch
// avoids a recursive chown on every mount of large data volumes.
//
// Optional sysctls are appended by the caller (the StatefulSet needs the
// low-port sysctl to bind 443; the bootstrapper Job does not).
func buildPodSecurityContext(sysctls ...corev1.Sysctl) *corev1.PodSecurityContext {
	fsGroupChangePolicy := corev1.FSGroupChangeOnRootMismatch

	return &corev1.PodSecurityContext{
		RunAsNonRoot:        pointer.Bool(true),
		RunAsUser:           pointer.Int64(common.RavenDBUID),
		RunAsGroup:          pointer.Int64(common.RavenDBGID),
		FSGroup:             pointer.Int64(common.RavenDBGID),
		FSGroupChangePolicy: &fsGroupChangePolicy,
		SeccompProfile:      &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		Sysctls:             sysctls,
	}
}
