package testutil

import (
	ravendbv1 "ravendb-operator/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BaseClusterLE builds a Mode=None secured cluster (tags a, b, c).
//
// Historically this used Mode=LetsEncrypt with fixed, publicly-trusted cert
// fixtures. Those certs expired and broke the suite; the e2e now runs Mode=None
// with a self-generated cluster CA + cluster server cert + admin client cert
// (see hack/gen-e2e-certs.sh), so it no longer depends on external cert
// validity. The name is kept to avoid churn across callers.
func BaseClusterLE(name string) *ravendbv1.RavenDBCluster {
	clusterCert := "ravendb-cert"
	caCert := "ravendb-ca-cert"
	storageClass := "local-path"

	return &ravendbv1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},

		Spec: ravendbv1.RavenDBClusterSpec{
			Image:                "ravendb/ravendb:6.2.11-ubuntu.22.04-x64",
			ImagePullPolicy:      "IfNotPresent",
			Mode:                 "None",
			LicenseSecretRef:     "ravendb-license",
			ClientCertSecretRef:  "ravendb-client-cert",
			ClusterCertSecretRef: &clusterCert,
			CACertSecretRef:      &caCert,

			Domain: "ravendb-operator-e2e.ravendb.run",

			Nodes: []ravendbv1.RavenDBNode{
				{Tag: "a", PublicServerUrl: "https://a.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://a-tcp.ravendb-operator-e2e.ravendb.run:443"},
				{Tag: "b", PublicServerUrl: "https://b.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://b-tcp.ravendb-operator-e2e.ravendb.run:443"},
				{Tag: "c", PublicServerUrl: "https://c.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://c-tcp.ravendb-operator-e2e.ravendb.run:443"},
			},
			Env: map[string]string{
				"RAVEN_Cluster_TimeBeforeMovingToRehabInSec": "10",
			},

			ExternalAccessConfiguration: &ravendbv1.ExternalAccessConfiguration{
				Type: "ingress-controller",

				IngressControllerExternalAccess: &ravendbv1.IngressControllerContext{IngressClassName: "nginx"},
			},

			StorageSpec: ravendbv1.StorageSpec{
				Data: ravendbv1.VolumeSpec{
					Size:             "10Gi",
					StorageClassName: &storageClass,
				},
			},
		},
	}
}

// BaseClusterLEDEF is the tags d, e, f variant of BaseClusterLE (Mode=None).
func BaseClusterLEDEF(name string) *ravendbv1.RavenDBCluster {
	clusterCert := "ravendb-cert"
	caCert := "ravendb-ca-cert"
	storageClass := "local-path"

	return &ravendbv1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ravendbv1.RavenDBClusterSpec{
			Image:                "ravendb/ravendb:6.2.11-ubuntu.22.04-x64",
			ImagePullPolicy:      "IfNotPresent",
			Mode:                 "None",
			LicenseSecretRef:     "ravendb-license",
			ClientCertSecretRef:  "ravendb-client-cert",
			ClusterCertSecretRef: &clusterCert,
			CACertSecretRef:      &caCert,
			Domain:               "ravendb-operator-e2e.ravendb.run",
			Nodes: []ravendbv1.RavenDBNode{
				{Tag: "d", PublicServerUrl: "https://d.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://d-tcp.ravendb-operator-e2e.ravendb.run:443"},
				{Tag: "e", PublicServerUrl: "https://e.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://e-tcp.ravendb-operator-e2e.ravendb.run:443"},
				{Tag: "f", PublicServerUrl: "https://f.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://f-tcp.ravendb-operator-e2e.ravendb.run:443"},
			},
			Env: map[string]string{
				"RAVEN_Cluster_TimeBeforeMovingToRehabInSec": "10",
			},
			ExternalAccessConfiguration: &ravendbv1.ExternalAccessConfiguration{
				Type: "ingress-controller",
				IngressControllerExternalAccess: &ravendbv1.IngressControllerContext{
					IngressClassName: "nginx",
				},
			},
			StorageSpec: ravendbv1.StorageSpec{
				Data: ravendbv1.VolumeSpec{
					Size:             "10Gi",
					StorageClassName: &storageClass,
				},
			},
		},
	}
}
