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

package v1

type RavenDBClusterSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="RavenDB Image"
	Image string `json:"image"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Always;IfNotPresent
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Image Pull Policy"
	ImagePullPolicy string `json:"imagePullPolicy"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=LetsEncrypt;None
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Security Mode"
	Mode ClusterMode `json:"mode"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^[^@\s]+@[^@\s]+\.[^@\s]+$`
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Let's Encrypt Email"
	Email *string `json:"email,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="License Secret"
	LicenseSecretRef string `json:"licenseSecretRef"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cluster Certificate Secret"
	ClusterCertSecretRef *string `json:"clusterCertSecretRef,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cluster Domain"
	Domain string `json:"domain"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Nodes"
	Nodes []RavenDBNode `json:"nodes,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Additional Environment Variables"
	Env map[string]string `json:"env,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="External Access Configuration"
	ExternalAccessConfiguration *ExternalAccessConfiguration `json:"externalAccessConfiguration,omitempty"`

	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Storage"
	StorageSpec StorageSpec `json:"storage"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Client Certificate Secret"
	ClientCertSecretRef string `json:"clientCertSecretRef"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="CA Certificate Secret"
	CACertSecretRef *string `json:"caCertSecretRef,omitempty"`

	// // +kubebuilder:validation:Optional
	// Sidecars []Sidecar `json:"sidecars,omitempty"`
}
