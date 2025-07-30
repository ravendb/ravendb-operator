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

package adapter

type ClusterAdapter interface {
	GetImage() string
	GetIpp() string
	SetIpp(string)
	GetMode() string
	GetEmail() string
	GetDomain() string
	GetEnv() map[string]string
	GetClusterCertsSecretRef() string
	GetLicenseSecretRef() string
	GetNodeTags() []string
	GetNodePublicUrls() []string
	GetNodeTcpUrls() []string
	GetNodeCertSecretRefs() []*string
	GetExternalAccessType() string
	GetIngressClassName() string
	GetIngressAnnotations() map[string]string
	IsIngressContextSet() bool
	IsAWSContextSet() bool
	IsAzureContextSet() bool
	IsExternalAccessSet() bool
	GetStorageDataStorageClassName() *string
	GetStorageDataAccessModes() []string
	GetStorageDataVAC() *string
	GetLogsRavenStorageClassName() *string
	GetLogsRavenAccessModes() []string
	GetLogsRavenVAC() *string
	GetLogsRavenPath() *string
	GetLogsAuditStorageClassName() *string
	GetLogsAuditAccessModes() []string
	GetLogsAuditVAC() *string
	GetLogsAuditPath() *string
	GetAdditionalVolumeNames() []string
	GetAdditionalVolumeMountPaths() []*string
	GetAdditionalVolumeSubPaths() []*string
	GetAdditionalVolumeSources() []map[string]bool
}
