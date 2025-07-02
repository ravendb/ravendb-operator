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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

type StorageSpec struct {

	// +kubebuilder:validation:Required
	Data VolumeSpec `json:"data"`

	// +kubebuilder:validation:Optional
	Logs *LogsSpec `json:"logs,omitempty"`

	// +kubebuilder:validation:Optional
	AdditionalVolumes []AdditionalVolume `json:"additionalVolumes,omitempty"`
}

type VolumeSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^\d+(Ei|Pi|Ti|Gi|Mi|Ki)$`
	Size string `json:"size"`

	// +kubebuilder:validation:Optional
	StorageClassName *string `json:"storageClassName,omitempty"`

	// todo: reject ReadWriteMany for poupular storage via dict
	// we should log a warning for unknown StorageClass names + RWX combo but still allow it. --> webhook
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Items(enum=ReadWriteOnce,ReadWriteMany)
	AccessModes []string `json:"accessModes,omitempty"`

	// +kubebuilder:validation:Optional
	VolumeAttributesClassName *string `json:"volumeAttributesClassName,omitempty"`
}

type LogsSpec struct {
	// +kubebuilder:validation:Optional
	RavenDB *LogSettings `json:"ravendb,omitempty"`

	// +kubebuilder:validation:Optional
	Audit *LogSettings `json:"audit,omitempty"`
}

type LogSettings struct {
	VolumeSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	Path *string `json:"path,omitempty"`
}

type AdditionalVolume struct {
	// TODO: verify uniqness of name across all volumes via webhook
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	MountPath string `json:"mountPath"`

	// +kubebuilder:validation:Optional
	SubPath string `json:"subPath,omitempty"`

	// +kubebuilder:validation:Required
	// TODO:  Only one field must be set -> webhook.
	VolumeSource VolumeSource `json:"volumeSource"`
}

type VolumeSource struct {
	// +kubebuilder:validation:Optional
	ConfigMap *corev1.ConfigMapVolumeSource `json:"configMap,omitempty"`

	// +kubebuilder:validation:Optional
	Secret *corev1.SecretVolumeSource `json:"secret,omitempty"`

	// +kubebuilder:validation:Optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
}
