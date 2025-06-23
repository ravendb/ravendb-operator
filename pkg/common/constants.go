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
	CertPath         = "/ravendb/certs/server.pfx"
	LicensePath      = "/ravendb/license/license.json"
	DataMountPath    = "/var/lib/ravendb/data"
	CertMountPath    = "/ravendb/certs"
	LicenseMountPath = "/ravendb/license"
)

// identifiers
const (
	App               = "ravendb"
	Manager           = "ravendb-operator"
	Prefix            = "ravendb-"
	HttpsPortName     = "https"
	TcpPortName       = "tcp"
	CertVolumeName    = "ravendb-cert"
	LicenseVolumeName = "ravendb-license"
)

// labels
const (
	LabelAppName   = "app.kubernetes.io/name"
	LabelInstance  = "app.kubernetes.io/instance"
	LabelManagedBy = "app.kubernetes.io/managed-by"
	LabelNodeTag   = "nodeTag"
	LabelApp       = "app"
)

// annotations
const (
	IngressSSLPassthroughAnnotation = "ingress.kubernetes.io/ssl-passthrough"
	NginxSSLPassthroughAnnotation   = "nginx.ingress.kubernetes.io/ssl-passthrough"
)

// internal ports
const (
	InternalHttpsPort = 443
	InternalTcpPort   = 38888
)

const (
	InternalHttpsUrl = "https://0.0.0.0:443"
	InternalTcpUrl   = "tcp://0.0.0.0:38888"
)

// other
const (
	NumOfReplicas     = 1
	CertExecTimeout   = "60"
	ClusterFQDNSuffix = ".ravendb.svc.cluster.local"
	ProtocolTcp       = "tcp://"
)
