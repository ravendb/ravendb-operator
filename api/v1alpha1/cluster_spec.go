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

type RavenDBClusterSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	ImagePullPolicy string `json:"imagePullPolicy"`

	// +kubebuilder:validation:Enum=LetsEncrypt;None
	Mode ClusterMode `json:"mode"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^[^@\s]+@[^@\s]+\.[^@\s]+$`
	Email string `json:"email,omitempty"`

	// +kubebuilder:validation:Required
	LicenseSecretRef string `json:"licenseSecretRef"`

	// +kubebuilder:validation:Optional
	ClusterCertSecretRef string `json:"certsSecretRef,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Domain string `json:"domain"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServerUrl string `json:"serverUrl"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ServerUrlTcp string `json:"serverUrlTcp"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	StorageSize string `json:"storageSize"`

	// +kubebuilder:validation:Optional
	Environment map[string]string `json:"environment,omitempty"` // env vars

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Nodes []RavenDBNode `json:"nodes,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	IngressClassName string `json:"ingressClassName"`

	// +kubebuilder:validation:Optional
	Env map[string]string `json:"env,omitempty"`
}
