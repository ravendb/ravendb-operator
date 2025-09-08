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

type ClusterMode string

const (
	ModeLetsEncrypt ClusterMode = "LetsEncrypt"
	ModeNone        ClusterMode = "None"
)

type ExternalAccessType string

const (
	ExternalAccessTypeAWS               ExternalAccessType = "aws-nlb"
	ExternalAccessTypeAzure             ExternalAccessType = "azure-lb"
	ExternalAccessTypeIngressController ExternalAccessType = "ingress-controller"
)

type ClusterPhase string

const (
	PhaseDeploying ClusterPhase = "Deploying"
	PhaseRunning   ClusterPhase = "Running"
	PhaseError     ClusterPhase = "Error"
)

type ClusterConditionType string

const (
	ConditionReady               ClusterConditionType = "Ready"
	ConditionProgressing         ClusterConditionType = "Progressing"
	ConditionDegraded            ClusterConditionType = "Degraded"
	ConditionCertificatesReady   ClusterConditionType = "CertificatesReady"
	ConditionLicensesValid       ClusterConditionType = "LicensesValid"
	ConditionStorageReady        ClusterConditionType = "StorageReady"
	ConditionExternalAccessReady ClusterConditionType = "ExternalAccessReady"
	ConditionNodesHealthy        ClusterConditionType = "NodesHealthy"
	ConditionBootstrapCompleted  ClusterConditionType = "BootstrapCompleted"
)

type ClusterConditionReason string

const (
	ReasonCompleted             ClusterConditionReason = "Completed"
	ReasonWaitingForPods        ClusterConditionReason = "WaitingForPods"
	ReasonPodsNotReady          ClusterConditionReason = "PodsNotReady"
	ReasonStatefulSetUpdating   ClusterConditionReason = "StatefulSetUpdating"
	ReasonIngressPendingAddress ClusterConditionReason = "IngressPendingAddress"
	ReasonLoadBalancerPending   ClusterConditionReason = "LoadBalancerPending"
	ReasonCertSecretMissing     ClusterConditionReason = "CertSecretMissing"
	ReasonLicenseSecretMissing  ClusterConditionReason = "LicenseSecretMissing"
	ReasonBootstrapJobRunning   ClusterConditionReason = "BootstrapJobRunning"
	ReasonBootstrapFailed       ClusterConditionReason = "BootstrapFailed"
	ReasonPVCNotBound           ClusterConditionReason = "PVCNotBound"
)
