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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type RavenDBNode struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=4
	Tag string `json:"tag"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PublicServerUrl string `json:"publicServerUrl"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	PublicServerUrlTcp string `json:"publicServerUrlTcp"`

	// +kubebuilder:validation:Optional
	CertSecretRef *string `json:"certSecretRef,omitempty"`
}

type RavenDBNodeStatusPhase string

const (
	NodeStatusCreated RavenDBNodeStatusPhase = "Created"
	NodeStatusFailed  RavenDBNodeStatusPhase = "Failed"
)

type RavenDBNodeStatus struct {
	Tag string `json:"tag"`

	// +kubebuilder:validation:Enum=Created;Failed
	Status             RavenDBNodeStatusPhase `json:"status"`
	LastAttemptedImage string                 `json:"lastAttemptedImage,omitempty"`
	LastError          string                 `json:"lastError,omitempty"`
	LastAttemptTime    metav1.Time            `json:"lastAttemptTime,omitempty"`
}
