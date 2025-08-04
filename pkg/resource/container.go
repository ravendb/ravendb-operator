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
	"k8s.io/utils/pointer"
)

type ContainerBuilder struct{}

func NewContainerBuilder() *ContainerBuilder {
	return &ContainerBuilder{}
}

func BuildRavenDBContainer(image string, env []corev1.EnvVar, ports []corev1.ContainerPort, mounts []corev1.VolumeMount, ipp corev1.PullPolicy) corev1.Container {
	return corev1.Container{
		Name:            common.App,
		Image:           image,
		Env:             env,
		Ports:           ports,
		VolumeMounts:    mounts,
		ImagePullPolicy: ipp,
		SecurityContext: &corev1.SecurityContext{RunAsUser: pointer.Int64(0)}, // TODO: to be removed
	}
}

// TODO: might use sidecars later
// func BuildSidecarContainers(sidecars []ravendbv1alpha1.Sidecar, additionalMounts []corev1.VolumeMount) []corev1.Container {
// 	var containers []corev1.Container

// 	for _, s := range sidecars {
// 		container := corev1.Container{
// 			Name:      s.Name,
// 			Image:     s.Image,
// 			Command:   s.Command,
// 			Env:       s.Env,
// 			Ports:     s.Ports,
// 			Resources: getResourcesOrEmpty(s.Resources),
// 		}
// 		containers = append(containers, container)
// 	}
// 	return containers
// }

// func getResourcesOrEmpty(res *corev1.ResourceRequirements) corev1.ResourceRequirements {
// 	if res == nil {
// 		return corev1.ResourceRequirements{}
// 	}
// 	return *res
// }
