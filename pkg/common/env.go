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

package common

import (
	"fmt"
	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

func BuildCommonEnvVars(cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) []corev1.EnvVar {

	ravendbNodeTcpEndpoint := fmt.Sprintf("%s%s%s%s:%d", ProtocolTcp, Prefix, node.Tag, ClusterFQDNSuffix, InternalTcpPort)
	return []corev1.EnvVar{
		{Name: "RAVEN_Setup_Mode", Value: string(cluster.Spec.Mode)},
		{Name: "RAVEN_License_Path", Value: LicensePath},
		{Name: "RAVEN_License_Eula_Accepted", Value: "true"},
		{Name: "RAVEN_PublicServerUrl", Value: node.PublicServerUrl},
		{Name: "RAVEN_PublicTcpUrl", Value: node.PublicServerUrlTcp},
		{Name: "RAVEN_ServerUrl", Value: InternalHttpsUrl},
		{Name: "RAVEN_ServerUrl_Tcp", Value: InternalTcpUrl},
		{Name: "RAVEN_PublicServerUrl_Tcp_Cluster", Value: ravendbNodeTcpEndpoint},
		{Name: "NODE_TAG", Value: node.Tag},
	}
}

func BuildSecureEnvVars(instance *ravendbv1alpha1.RavenDBCluster) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "RAVEN_Security_Certificate_Load_Exec", Value: GetCertScriptPath},
		{Name: "RAVEN_Security_Certificate_Change_Exec", Value: UpdateCertScriptPath},
		{Name: "RAVEN_Security_Certificate_Exec_TimeoutInSec", Value: CertExecTimeout},
	}
}

func BuildSecureLetsEncryptEnvVars(instance *ravendbv1alpha1.RavenDBCluster) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "RAVEN_Security_Certificate_Load_Exec", Value: GetCertScriptPath},
		{Name: "RAVEN_Security_Certificate_Change_Exec", Value: UpdateCertScriptPath},
		{Name: "RAVEN_Security_Certificate_Exec_TimeoutInSec", Value: CertExecTimeout},
		{Name: "RAVEN_Security_Certificate_LetsEncrypt_Email", Value: *instance.Spec.Email},
	}
}

func BuildAdditionalEnvVars(cluster *ravendbv1alpha1.RavenDBCluster) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	for k, v := range cluster.Spec.Env {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}
	return envVars
}

func BuildClusterBootstrapperEnvVars(leaderURL string, memberURLs []string, allURLs []string, allTags []string, tcpHosts []string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "LEADER_URL", Value: leaderURL},
		{Name: "MEMBER_URLS", Value: strings.Join(memberURLs, " ")},
		{Name: "URLS", Value: strings.Join(allURLs, " ")},
		{Name: "TAGS", Value: strings.Join(allTags, " ")},
		{Name: "TCP_HOSTS", Value: strings.Join(tcpHosts, " ")},
	}
}
