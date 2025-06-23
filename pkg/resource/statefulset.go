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
	"context"
	"fmt"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"ravendb-operator/pkg/common"

	"k8s.io/utils/pointer"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

type StatefulSetBuilder struct{}

func NewStatefulSetBuilder() PerNodeBuilder {
	return &StatefulSetBuilder{}
}

func (b *StatefulSetBuilder) Build(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) (client.Object, error) {
	return BuildStatefulSet(cluster, node)
}

func BuildStatefulSet(cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) (*appsv1.StatefulSet, error) {
	stsName := fmt.Sprintf("%s%s", common.Prefix, node.Tag)

	replicas := int32(common.NumOfReplicas)
	labels := buildStatefulsetLabels(cluster, node)
	selector := &metav1.LabelSelector{MatchLabels: buildStatefulsetSelector(node)}
	annotations := buildStatefulsetAnnotations()
	ports := buildPorts()
	volumes := buildVolumes(cluster, node)
	volumeMounts := buildVolumeMounts()
	volumeClaims := buildVolumeClaims(cluster)
	envVars, err := buildEnvVars(cluster, node)
	if err != nil {
		return nil, err
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        stsName,
			Namespace:   cluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: common.App,
			Replicas:    &replicas,
			Selector:    selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:         common.App,
							Image:        cluster.Spec.Image,
							Env:          envVars,
							Ports:        ports,
							VolumeMounts: volumeMounts,
							// TODO: to be removed
							SecurityContext: &corev1.SecurityContext{RunAsUser: pointer.Int64(0)},
						},
					},
					Volumes: volumes,
				},
			},
			VolumeClaimTemplates: volumeClaims,
		},
	}

	return sts, nil
}

func buildStatefulsetSelector(node ravendbv1alpha1.RavenDBNode) map[string]string {
	return map[string]string{
		common.LabelNodeTag: node.Tag}
}
func buildStatefulsetLabels(cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) map[string]string {
	return map[string]string{
		common.LabelAppName:   common.App,
		common.LabelManagedBy: common.Manager,
		common.LabelInstance:  cluster.Name,
		common.LabelNodeTag:   node.Tag,
	}
}

func buildStatefulsetAnnotations() map[string]string {
	return map[string]string{
		common.IngressSSLPassthroughAnnotation: "true",
	}
}

func buildEnvVars(cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) ([]corev1.EnvVar, error) {
	env := common.BuildCommonEnvVars(cluster, node)

	switch cluster.Spec.Mode {
	case ravendbv1alpha1.ModeLetsEncrypt:
		env = append(env, common.BuildSecureLetsEncryptEnvVars(cluster)...)
	case ravendbv1alpha1.ModeNone:
		env = append(env, common.BuildSecureEnvVars(cluster)...)
	default:
		return nil, fmt.Errorf("unsupported cluster mode: %s", cluster.Spec.Mode)
	}

	env = append(env, common.BuildAdditionalEnvVars(cluster)...)

	return env, nil
}

func buildPorts() []corev1.ContainerPort {
	//TODO take this port as an argument from specs and validates (also across nodes)via webhooks !!!
	return []corev1.ContainerPort{
		{Name: common.HttpsPortName, ContainerPort: 443},
		{Name: common.TcpPortName, ContainerPort: 38888},
	}
}

func buildVolumes(cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) []corev1.Volume {

	////////////////////////////////////////////////////////////////////////////
	// to be removed - validation and fallback should be done in webhooks
	certSecretName := node.CertsSecretRef
	if certSecretName == "" {
		certSecretName = cluster.Spec.ClusterCertSecretRef
	}
	if certSecretName == "" {
		panic("no cert secret defined")
	}
	///////////////////////////////////////////////////////////////////////////
	return []corev1.Volume{
		{Name: common.CertVolumeName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: node.CertsSecretRef}}},
		{Name: common.LicenseVolumeName, VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: cluster.Spec.LicenseSecretRef}}},
	}
}

func buildVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: common.App, MountPath: common.DataMountPath},
		{Name: common.CertVolumeName, MountPath: common.CertMountPath},
		{Name: common.LicenseVolumeName, MountPath: common.LicenseMountPath},
	}
}

func buildVolumeClaims(cluster *ravendbv1alpha1.RavenDBCluster) []corev1.PersistentVolumeClaim {
	return []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.App,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: k8sresource.MustParse(cluster.Spec.StorageSize),
					},
				},
			},
		},
	}
}
