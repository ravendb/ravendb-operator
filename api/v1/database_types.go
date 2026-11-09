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
// +kubebuilder:printcolumn:name="Database",type=string,JSONPath=`.spec.databaseName`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +operator-sdk:csv:customresourcedefinitions:displayName="RavenDB Database"

type RavenDBDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RavenDBDatabaseSpec `json:"spec,omitempty"`

	// The default makes a new resource read as not yet observed, so kstatus (helm --wait) waits for the operator.
	// +kubebuilder:default={observedGeneration: 0}
	Status RavenDBDatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type RavenDBDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RavenDBDatabase `json:"items"`
}

type RavenDBDatabaseSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterRef is immutable"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cluster"
	ClusterRef ClusterReference `json:"clusterRef"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="databaseName is immutable"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Database Name"
	DatabaseName string `json:"databaseName"`

	// Number of nodes that hold the database. Applied when the database is created.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="replicationFactor is immutable"
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Replication Factor"
	ReplicationFactor int32 `json:"replicationFactor,omitempty"`

	// Delete removes the database and its data when this resource is deleted. Retain keeps it.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=Retain
	// +kubebuilder:validation:Enum=Retain;Delete
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Deletion Policy"
	DeletionPolicy DeletionPolicy `json:"deletionPolicy,omitempty"`
}

type RavenDBDatabaseStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Observed Generation"
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Conditions"
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ClusterReference names a RavenDBCluster in the resource's own namespace.
type ClusterReference struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
}

type DeletionPolicy string

const (
	DeletionPolicyRetain DeletionPolicy = "Retain"
	DeletionPolicyDelete DeletionPolicy = "Delete"
)
