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
	"fmt"
	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VolumeBuilder struct{}

func NewVolumeBuilder() *VolumeBuilder {
	return &VolumeBuilder{}
}

func buildPersistentVolumeClaim(name string, size string, className *string, accessModes []string, vaClass *string) corev1.PersistentVolumeClaim {

	modes, err := ConvertAccessModes(accessModes)
	if err != nil {
		panic(fmt.Errorf("invalid accessModes for volume %s: %w", name, err))
	}

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: modes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: k8sresource.MustParse(size),
				},
			},
		},
	}

	// TODO: check and fallback in webhooks
	if className != nil {
		pvc.Spec.StorageClassName = className
	}

	// TODO: check and fallback in webhooks
	// todo: actualy test it
	if vaClass != nil {
		//pvc.Spec.VolumeAttributesClassName = vaClass
		// this is an alpha feature, should be enable with feature gate
		fmt.Printf("will use VolumeAttributesClassName: %s\n", *vaClass)

	}

	return pvc
}

func buildPVCVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: name,
			},
		},
	}
}

func buildSecretVolume(name string, secretName string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	}
}

func buildVolumeMount(name string, mountPath string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		MountPath: mountPath,
	}
}

func buildAdditionalVolumes(additionalVols []ravendbv1alpha1.AdditionalVolume) []corev1.Volume {
	var volumes []corev1.Volume

	for _, av := range additionalVols {
		vol := corev1.Volume{
			Name: av.Name,
		}

		// TODO: check should be removed when webhook is implemented
		switch {
		case av.VolumeSource.ConfigMap != nil:
			vol.VolumeSource.ConfigMap = av.VolumeSource.ConfigMap

		case av.VolumeSource.Secret != nil:
			vol.VolumeSource.Secret = av.VolumeSource.Secret

		case av.VolumeSource.PersistentVolumeClaim != nil:
			vol.VolumeSource.PersistentVolumeClaim = av.VolumeSource.PersistentVolumeClaim

		default:
			// TODO: ignore invalid additional volumes -> webhook
			continue
		}

		volumes = append(volumes, vol)
	}

	return volumes
}

func ConvertAccessModes(input []string) ([]corev1.PersistentVolumeAccessMode, error) {
	// 	// TODO: check and fallback in webhooks
	if len(input) == 0 {
		return []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, nil
	}

	var result []corev1.PersistentVolumeAccessMode
	for _, mode := range input {
		switch corev1.PersistentVolumeAccessMode(mode) {
		case corev1.ReadWriteOnce, corev1.ReadWriteMany:
			result = append(result, corev1.PersistentVolumeAccessMode(mode))
		default:
			// TODO: Move validation to webhook and disallow fallback
			return nil, fmt.Errorf("invalid accessMode: %q", mode)
		}
	}

	return result, nil
}
