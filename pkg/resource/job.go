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
	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"ravendb-operator/pkg/common"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

type JobBuilder struct{}

func NewJobBuilder() PerClusterBuilder {
	return &JobBuilder{}
}

func (b *JobBuilder) Build(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster) (client.Object, error) {
	return BuildJob(cluster)
}

func BuildJob(cluster *ravendbv1alpha1.RavenDBCluster) (*batchv1.Job, error) {
	jobName := common.RavenDbBootstrapperJob
	backoff := int32(0)

	labels := buildJobLabels(cluster)
	volumes := buildJobVolumes(cluster)
	volumeMounts := buildJobVolumeMounts(cluster)
	envVars, _ := buildJobEnvVars(cluster)
	containers := buildJobContainers(cluster.Spec.Image, volumeMounts, envVars)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoff,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					Volumes:            volumes,
					Containers:         containers,
					ServiceAccountName: common.RavenDbNodeServiceAccount,
				},
			},
		},
	}
	return job, nil
}

func buildJobLabels(cluster *ravendbv1alpha1.RavenDBCluster) map[string]string {
	return map[string]string{
		common.LabelAppName:   common.App,
		common.LabelManagedBy: common.Manager,
		common.LabelInstance:  cluster.Name,
	}
}

func buildJobContainers(image string, vMounts []corev1.VolumeMount, env []corev1.EnvVar) []corev1.Container {
	bootstrapperContainer := BuildClusterBootstrapperContainer(image, vMounts, env)

	return []corev1.Container{bootstrapperContainer}

}

func buildJobVolumes(cluster *ravendbv1alpha1.RavenDBCluster) []corev1.Volume {
	var vols []corev1.Volume

	vols = append(vols, buildSecretVolume(common.ClientCertVolumeName, cluster.Spec.ClientCertSecretRef))

	serverCert := getServerCertSecretName(cluster)
	vols = append(vols, buildSecretVolume(common.CertVolumeName, serverCert))

	if cluster.Spec.CACertSecretRef != nil {
		vols = append(vols, buildSecretVolume(common.CACertVolumeName, *cluster.Spec.CACertSecretRef))
	}

	vols = append(vols, buildConfigMapVolume(
		common.BootstrapperHookVolumeName,
		common.BootstrapperHookConfigMap,
		map[string]string{
			common.InitClusterHookKey:               common.InitClusterHookKey,
			common.CheckNodesDiscoverabilityHookKey: common.CheckNodesDiscoverabilityHookKey,
		},
		common.ConfigMapExecMode,
	))

	return vols
}

func buildJobVolumeMounts(cluster *ravendbv1alpha1.RavenDBCluster) []corev1.VolumeMount {
	var vMounts []corev1.VolumeMount

	cvm := buildVolumeMount(common.ClientCertVolumeName, common.ClientCertMountPath)
	cvm.ReadOnly = true
	vMounts = append(vMounts, cvm)

	svm := buildVolumeMount(common.CertVolumeName, common.CertMountPath)
	svm.ReadOnly = true
	vMounts = append(vMounts, svm)

	if cluster.Spec.CACertSecretRef != nil {
		cavm := buildVolumeMount(common.CACertVolumeName, common.CACertMountPath)
		cavm.ReadOnly = true
		vMounts = append(vMounts, cavm)
	}

	InitClusterScriptMount := buildVolumeMount(common.BootstrapperHookVolumeName, common.InitClusterScriptPath)
	InitClusterScriptMount.SubPath = common.InitClusterHookKey
	InitClusterScriptMount.ReadOnly = true

	CheckNodesDiscoverabilityScriptMount := buildVolumeMount(common.BootstrapperHookVolumeName, common.CheckNodesDiscoverabilityScriptPath)
	CheckNodesDiscoverabilityScriptMount.SubPath = common.CheckNodesDiscoverabilityHookKey
	CheckNodesDiscoverabilityScriptMount.ReadOnly = true

	vMounts = append(vMounts, InitClusterScriptMount, CheckNodesDiscoverabilityScriptMount)

	return vMounts
}

func buildJobEnvVars(cluster *ravendbv1alpha1.RavenDBCluster) ([]corev1.EnvVar, error) {

	leader := cluster.Spec.Nodes[0] // first node is leader
	leaderURL := leader.PublicServerUrl
	leaderTag := leader.Tag

	var memberURLs []string
	var memberTags []string
	var tcpHosts []string

	for i, n := range cluster.Spec.Nodes {

		if tcpHost, ok := strings.CutPrefix(n.PublicServerUrlTcp, common.ProtocolTcp); ok {
			tcpHosts = append(tcpHosts, tcpHost)
		}

		if i == 0 {
			continue
		}
		memberURLs = append(memberURLs, n.PublicServerUrl)
		memberTags = append(memberTags, n.Tag)
	}

	allURLs := append([]string{leaderURL}, memberURLs...)
	allTags := append([]string{leaderTag}, memberTags...)
	env := common.BuildClusterBootstrapperEnvVars(leaderURL, memberURLs, allURLs, allTags, tcpHosts)

	return env, nil
}

func containsTag(tagsList *[]string, tag string) bool {
	if tagsList == nil {
		return false
	}
	for _, w := range *tagsList {
		if w == tag {
			return true
		}
	}
	return false
}

func getServerCertSecretName(cluster *ravendbv1alpha1.RavenDBCluster) string {

	switch cluster.Spec.Mode {
	case ravendbv1alpha1.ModeLetsEncrypt:
		return *cluster.Spec.Nodes[0].CertSecretRef

	case ravendbv1alpha1.ModeNone:
		if cluster.Spec.ClusterCertSecretRef != nil {
			return *cluster.Spec.ClusterCertSecretRef
		}
	}

	return ""
}
