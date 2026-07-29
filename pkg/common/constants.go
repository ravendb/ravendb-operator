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

// paths
const (
	LicensePath                         = "/ravendb/license/license.json"
	DataMountPath                       = "/var/lib/ravendb/data"
	CertMountPath                       = "/ravendb/certs"
	ClientCertMountPath                 = "/ravendb/client-certs"
	CACertMountPath                     = "/ravendb/ca-cert"
	LicenseMountPath                    = "/ravendb/license"
	LogsMountPath                       = "/var/log/ravendb/logs"
	AuditMountPath                      = "/var/log/ravendb/audit"
	CertSourcePath                      = "ravendb/cert-source"
	UpdateCertScriptPath                = "/ravendb/scripts/update-cert.sh"
	GetCertScriptPath                   = "/ravendb/scripts/get-server-cert.sh"
	InitClusterScriptPath               = "/ravendb/scripts/init-cluster.sh"
	CheckNodesDiscoverabilityScriptPath = "/ravendb/scripts/check-nodes-discoverability.sh"
)

// identifiers
const (
	App                        = "ravendb"
	Manager                    = "ravendb-operator"
	Prefix                     = "ravendb-"
	HttpsPortName              = "https"
	TcpPortName                = "tcp"
	CertVolumeName             = "ravendb-cert"
	LicenseVolumeName          = "ravendb-license"
	DataVolumeName             = "ravendb-data"
	LogsVolumeName             = "ravendb-logs"
	AuditVolumeName            = "ravendb-audit"
	ClientCertVolumeName       = "ravendb-client-cert"
	CACertVolumeName           = "ravendb-ca-cert"
	CertHookVolumeName         = "ravendb-cert-hook"
	BootstrapperHookVolumeName = "ravendb-bootstrapper-hook"
	RavenDbNodeServiceAccount  = "ravendb-ops-sa"
	RavenDbBootstrapperJob     = "ravendb-cluster-init"
)

// labels
const (
	LabelAppName      = "app.kubernetes.io/name"
	LabelInstance     = "app.kubernetes.io/instance"
	LabelManagedBy    = "app.kubernetes.io/managed-by"
	LabelNodeTag      = "nodeTag"
	LabelApp          = "app"
	TopologyZoneLabel = "topology.kubernetes.io/zone"
)

// annotations
const (
	IngressSSLPassthroughAnnotation         = "ingress.kubernetes.io/ssl-passthrough"
	NginxSSLPassthroughAnnotation           = "nginx.ingress.kubernetes.io/ssl-passthrough"
	HaproxySSLPassthroughAnnotation         = "haproxy.org/ssl-passthrough"
	AWSLoadBalancerTypeAnnotation           = "service.beta.kubernetes.io/aws-load-balancer-type"
	AWSLoadBalancerSchemeAnnotation         = "service.beta.kubernetes.io/aws-load-balancer-scheme"
	AWSLoadBalancerNLBTargetTypeAnnotation  = "service.beta.kubernetes.io/aws-load-balancer-nlb-target-type"
	AWSLoadBalancerEIPAllocationsAnnotation = "service.beta.kubernetes.io/aws-load-balancer-eip-allocations"
	AWSLoadBalancerSubnetsAnnotation        = "service.beta.kubernetes.io/aws-load-balancer-subnets"
	UpgradeImageAnnotation                  = "ravendb.ravendb.io/upgrade-image"
	PodTemplateRevisionAnnotation           = "ravendb.ravendb.io/pod-template-revision"
	UpgradePreWaitAnnotation                = "ravendb.io/upgrade-pre-wait"
	UpgradePostWaitAnnotation               = "ravendb.io/upgrade-post-wait"
	UpgradePingIntervalAnnotation           = "ravendb.io/upgrade-ping-interval"
	UpgradeDBIntervalAnnotation             = "ravendb.io/upgrade-db-interval"
)

// CurrentPodTemplateRevision is bumped when a PodTemplate change must be
// applied during the next coordinated rollout or requires replacement of a
// terminal bootstrap Job. A missing annotation is the legacy revision 0, so
// "1" is the first explicitly versioned PodTemplate.
const CurrentPodTemplateRevision = "1"

// internal ports
const (
	InternalHttpsPort = 443
	InternalTcpPort   = 38888
)

// runtime identity
//
// The RavenDB image declares `USER ravendb` (uid 999, gid 999) and ships its
// data dir group-owned by 999. Kubernetes cannot infer that identity, so we
// restate it here as a single source of truth feeding both the container
// security context (runAsUser/runAsGroup) and the pod fsGroup. A CI guard
// (hack/verify-image-uid.sh) asserts the supported images still match these,
// so a UID change upstream fails in CI instead of on a user's cluster.
const (
	RavenDBUID int64 = 999
	RavenDBGID int64 = 999
)

// ingress controller types
const (
	IngressControllerTypeNginx   = "nginx"
	IngressControllerTypeTraefik = "traefik"
	IngressControllerTypeHaproxy = "haproxy"
)

const (
	InternalHttpsUrl = "https://0.0.0.0:443"
	InternalTcpUrl   = "tcp://0.0.0.0:38888"
)

// other
const (
	NumOfReplicas                    = 1
	ConfigMapExecMode                = 0755
	CertExecTimeout                  = "60"
	ProtocolTcp                      = "tcp://"
	UpdateCertHookKey                = "update-cert.sh"
	GetCertHookKey                   = "get-server-cert.sh"
	CertHookConfigMap                = "ravendb-cert-hook"
	InitClusterHookKey               = "init-cluster.sh"
	CheckNodesDiscoverabilityHookKey = "check-nodes-discoverability.sh"
	BootstrapperHookConfigMap        = "ravendb-bootstrapper-hook"
)
