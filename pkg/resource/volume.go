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
	"sort"

	corev1 "k8s.io/api/core/v1"
	k8sresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type VolumeBuilder struct{}

func NewVolumeBuilder() *VolumeBuilder {
	return &VolumeBuilder{}
}

func buildPersistentVolumeClaim(
	name string,
	size string,
	className *string,
	accessModes *[]string,
	vaClass *string,
) corev1.PersistentVolumeClaim {

	if accessModes == nil {
		accessModes = &[]string{"ReadWriteOnce"}
	}

	modes, err := ConvertAccessModes(*accessModes)
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

	if className != nil {
		pvc.Spec.StorageClassName = className
	}

	if vaClass != nil {
		pvc.Spec.VolumeAttributesClassName = vaClass
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

		vol.VolumeSource = corev1.VolumeSource{
			ConfigMap:             av.VolumeSource.ConfigMap,
			Secret:                av.VolumeSource.Secret,
			PersistentVolumeClaim: av.VolumeSource.PersistentVolumeClaim,
		}

		volumes = append(volumes, vol)
	}

	return volumes
}

func ConvertAccessModes(input []string) ([]corev1.PersistentVolumeAccessMode, error) {
	result := make([]corev1.PersistentVolumeAccessMode, len(input))
	for i, mode := range input {
		result[i] = corev1.PersistentVolumeAccessMode(mode)
	}
	return result, nil
}

func buildConfigMapVolume(name string, cfgmName string, keyToPath map[string]string, mode int32) corev1.Volume {
	var items []corev1.KeyToPath

	keys := make([]string, 0, len(keyToPath))
	for k := range keyToPath {
		keys = append(keys, k)
	}

	// required !!! cfgms vols can be mounted in different order which trigger the reconciler to terminate and restart the containers for no real reason
	sort.Strings(keys)

	for _, k := range keys {
		items = append(items, corev1.KeyToPath{
			Key:  k,
			Path: keyToPath[k],
		})
	}

	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cfgmName,
				},
				Items:       items,
				DefaultMode: &mode,
			},
		},
	}
}
