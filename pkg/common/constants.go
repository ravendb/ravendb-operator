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
	CertPath             = "/ravendb/certs/server.pfx"
	LicensePath          = "/ravendb/license/license.json"
	DataMountPath        = "/var/lib/ravendb/data"
	CertMountPath        = "/ravendb/certs"
	LicenseMountPath     = "/ravendb/license"
	LogsMountPath        = "/var/log/ravendb/logs"
	AuditMountPath       = "/var/log/ravendb/audit"
	CertSourcePath       = "ravendb/cert-source"
	UpdateCertScriptPath = "/ravendb/scripts/update-cert.sh"
	GetCertScriptPath    = "/ravendb/scripts/get-server-cert.sh"
)

// identifiers
const (
	App                       = "ravendb"
	Manager                   = "ravendb-operator"
	Prefix                    = "ravendb-"
	HttpsPortName             = "https"
	TcpPortName               = "tcp"
	CertVolumeName            = "ravendb-cert"
	LicenseVolumeName         = "ravendb-license"
	DataVolumeName            = "ravendb-data"
	LogsVolumeName            = "ravendb-logs"
	AuditVolumeName           = "ravendb-audit"
	CertHookVolumeName        = "ravendb-cert-hook"
	RavenDbNodeServiceAccount = "ravendb-node"
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
)

// internal ports
const (
	InternalHttpsPort = 443
	InternalTcpPort   = 38888
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
	NumOfReplicas     = 1
	ConfigMapExecMode = 0755
	CertExecTimeout   = "60"
	ClusterFQDNSuffix = ".ravendb.svc.cluster.local"
	ProtocolTcp       = "tcp://"
	UpdateCertHookKey = "update-cert.sh"
	GetCertHookKey    = "get-server-cert.sh"
	CertHookConfigMap = "ravendb-cert-hook"
)
