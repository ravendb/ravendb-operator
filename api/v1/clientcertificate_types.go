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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Secret",type=string,JSONPath=`.spec.secretName`
// +kubebuilder:printcolumn:name="Clearance",type=string,JSONPath=`.spec.clearance`
// +kubebuilder:printcolumn:name="Expires",type=string,JSONPath=`.status.notAfter`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +operator-sdk:csv:customresourcedefinitions:displayName="RavenDB Client Certificate",resources={{Secret,v1,<secretName>}}

type RavenDBClientCertificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RavenDBClientCertificateSpec `json:"spec,omitempty"`

	// The default makes a new resource read as not yet observed, so kstatus (helm --wait) waits for the operator.
	// +kubebuilder:default={observedGeneration: 0}
	Status RavenDBClientCertificateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type RavenDBClientCertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RavenDBClientCertificate `json:"items"`
}

type RavenDBClientCertificateSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterRef is immutable"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cluster"
	ClusterRef ClusterReference `json:"clusterRef"`

	// Secret the operator creates with client.pfx (no password), plus ca.crt when the cluster uses a private CA.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="secretName is immutable"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Secret Name"
	SecretName string `json:"secretName"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=ValidUser
	// +kubebuilder:validation:Enum=ClusterAdmin;Operator;ValidUser
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Security Clearance"
	Clearance SecurityClearance `json:"clearance,omitempty"`

	// Access per database name. Only a ValidUser clearance needs it.
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Database Permissions"
	Permissions map[string]DatabaseAccess `json:"permissions,omitempty"`

	// Lifetime of each issued certificate. The operator renews a certificate after two thirds of this time, or of
	// its own lifetime when that is shorter.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="8760h"
	// +kubebuilder:validation:XValidation:rule="duration(self) >= duration('1h')",message="validity must be at least 1h"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Validity"
	Validity metav1.Duration `json:"validity,omitzero"`
}

type RavenDBClientCertificateStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Observed Generation"
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Conditions"
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Thumbprint of the certificate in the Secret.
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Thumbprint"
	Thumbprint string `json:"thumbprint,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Expires"
	NotAfter *metav1.Time `json:"notAfter,omitempty"`
}

type SecurityClearance string

const (
	ClearanceClusterAdmin SecurityClearance = "ClusterAdmin"
	ClearanceOperator     SecurityClearance = "Operator"
	ClearanceValidUser    SecurityClearance = "ValidUser"
)

// +kubebuilder:validation:Enum=Read;ReadWrite;Admin
type DatabaseAccess string

const (
	DatabaseAccessRead      DatabaseAccess = "Read"
	DatabaseAccessReadWrite DatabaseAccess = "ReadWrite"
	DatabaseAccessAdmin     DatabaseAccess = "Admin"
)
