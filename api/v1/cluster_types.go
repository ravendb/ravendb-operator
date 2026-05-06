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
// +operator-sdk:csv:customresourcedefinitions:displayName="Raven DBCluster",resources={{StatefulSet,v1,ravendb-<nodeTag>},{Service,v1,ravendb-<nodeTag>},{Ingress,v1,ravendb},{ServiceAccount,v1,ravendb-ops-sa},{Job,v1,ravendb-cluster-init},{ConfigMap,v1,ravendb-bootstrapper-hook},{ConfigMap,v1,ravendb-cert-hook}}

type RavenDBCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RavenDBClusterSpec   `json:"spec,omitempty"`
	Status RavenDBClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type RavenDBClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RavenDBCluster `json:"items"`
}

type RavenDBClusterStatus struct {
	// +kubebuilder:validation:Enum=Deploying;Running;Error
	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Cluster Phase"
	Phase ClusterPhase `json:"phase,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Status Message"
	Message string `json:"message,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Observed Generation"
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Node Statuses"
	Nodes []RavenDBNodeStatus `json:"nodes,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,displayName="Cluster Conditions"
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
