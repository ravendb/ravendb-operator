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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

	volumeClaims := BuildPVCs(cluster)
	volumes := buildVolumes(cluster, node)
	volumeMounts := buildVolumeMounts(cluster)

	envVars, _ := buildEnvVars(cluster, node)

	ipp := corev1.PullPolicy(cluster.Spec.ImagePullPolicy)

	containers := buildContainers(cluster.Spec.Image, envVars, ports, volumeMounts, ipp, cluster)

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
					Containers: containers,
					Volumes:    volumes,
				},
			},
			VolumeClaimTemplates: volumeClaims,
		},
	}

	return sts, nil
}

func buildContainers(image string, env []corev1.EnvVar, ports []corev1.ContainerPort, mounts []corev1.VolumeMount, ipp corev1.PullPolicy, cluster *ravendbv1alpha1.RavenDBCluster) []corev1.Container {
	rdbContainer := BuildRavenDBContainer(image, env, ports, mounts, ipp)

	// TODO: might use sidecars later
	//sideCarContainers := BuildSidecarContainers(cluster.Spec.Sidecars, nil)
	//containers := append([]corev1.Container{rdbContainer}, sideCarContainers...)
	//return containers

	return []corev1.Container{rdbContainer}

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
	}

	env = append(env, common.BuildAdditionalEnvVars(cluster)...)

	return env, nil
}

func buildPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: common.HttpsPortName, ContainerPort: 443},
		{Name: common.TcpPortName, ContainerPort: 38888},
	}
}

func buildVolumes(cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) []corev1.Volume {

	volumes := []corev1.Volume{
		buildPVCVolume(common.DataVolumeName),
		buildSecretVolume(common.CertVolumeName, *node.CertSecretRef),
		buildSecretVolume(common.LicenseVolumeName, cluster.Spec.LicenseSecretRef),
	}

	if cluster.Spec.StorageSpec.Logs != nil {
		if cluster.Spec.StorageSpec.Logs.RavenDB != nil {
			volumes = append(volumes, buildPVCVolume(common.LogsVolumeName))
		}
		if cluster.Spec.StorageSpec.Logs.Audit != nil {
			volumes = append(volumes, buildPVCVolume(common.AuditVolumeName))
		}
	}

	if cluster.Spec.StorageSpec.AdditionalVolumes != nil {
		volumes = append(volumes, buildAdditionalVolumes(*cluster.Spec.StorageSpec.AdditionalVolumes)...)
	}

	return volumes
}

func buildVolumeMounts(cluster *ravendbv1alpha1.RavenDBCluster) []corev1.VolumeMount {
	vMounts := []corev1.VolumeMount{
		buildVolumeMount(common.DataVolumeName, common.DataMountPath),
		buildVolumeMount(common.CertVolumeName, common.CertMountPath),
		buildVolumeMount(common.LicenseVolumeName, common.LicenseMountPath),
	}

	if logs := cluster.Spec.StorageSpec.Logs; logs != nil {
		if logs.RavenDB != nil {
			path := common.LogsMountPath
			if logs.RavenDB.Path != nil {
				path = *logs.RavenDB.Path
			}
			vMounts = append(vMounts, buildVolumeMount(common.LogsVolumeName, path))
		}

		if logs.Audit != nil {
			path := common.AuditMountPath
			if logs.Audit.Path != nil {
				path = *logs.Audit.Path
			}
			vMounts = append(vMounts, buildVolumeMount(common.AuditVolumeName, path))
		}
	}

	if cluster.Spec.StorageSpec.AdditionalVolumes != nil {
		for _, av := range *cluster.Spec.StorageSpec.AdditionalVolumes {
			mount := corev1.VolumeMount{
				Name:      av.Name,
				MountPath: av.MountPath,
			}
			if av.SubPath != nil {
				mount.SubPath = *av.SubPath
			}
			vMounts = append(vMounts, mount)
		}
	}

	return vMounts
}

func BuildPVCs(cluster *ravendbv1alpha1.RavenDBCluster) []corev1.PersistentVolumeClaim {
	var pvcs []corev1.PersistentVolumeClaim

	data := cluster.Spec.StorageSpec.Data
	pvcs = append(pvcs, buildPersistentVolumeClaim(
		common.DataVolumeName,
		data.Size,
		data.StorageClassName,
		data.AccessModes,
		data.VolumeAttributesClassName,
	))

	logs := cluster.Spec.StorageSpec.Logs
	if logs != nil {
		if logs.RavenDB != nil {
			pvcs = append(pvcs, buildPersistentVolumeClaim(
				common.LogsVolumeName,
				logs.RavenDB.Size,
				logs.RavenDB.StorageClassName,
				logs.RavenDB.AccessModes,
				logs.RavenDB.VolumeAttributesClassName,
			))
		}

		if logs.Audit != nil {
			pvcs = append(pvcs, buildPersistentVolumeClaim(
				common.AuditVolumeName,
				logs.Audit.Size,
				logs.Audit.StorageClassName,
				logs.Audit.AccessModes,
				logs.Audit.VolumeAttributesClassName,
			))
		}
	}

	return pvcs
}
