package testutil

import (
	ravendbv1 "ravendb-operator/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BaseClusterLE(name string) *ravendbv1.RavenDBCluster {
	email := "user@ravendb.net"
	certA, certB, certC := "ravendb-certs-a", "ravendb-certs-b", "ravendb-certs-c"
	storageClass := "local-path"

	return &ravendbv1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ravendbv1.RavenDBClusterSpec{
			Image:               "ravendb/ravendb:6.2.9-ubuntu.22.04-x64",
			ImagePullPolicy:     "IfNotPresent",
			Mode:                "LetsEncrypt",
			Email:               &email,
			LicenseSecretRef:    "ravendb-license",
			ClientCertSecretRef: "ravendb-client-cert",
			Domain:              "ravendbe2e.development.run",
			Nodes: []ravendbv1.RavenDBNode{
				{Tag: "a", PublicServerUrl: "https://a.ravendbe2e.development.run:443", PublicServerUrlTcp: "tcp://a-tcp.ravendbe2e.development.run:443", CertSecretRef: &certA},
				{Tag: "b", PublicServerUrl: "https://b.ravendbe2e.development.run:443", PublicServerUrlTcp: "tcp://b-tcp.ravendbe2e.development.run:443", CertSecretRef: &certB},
				{Tag: "c", PublicServerUrl: "https://c.ravendbe2e.development.run:443", PublicServerUrlTcp: "tcp://c-tcp.ravendbe2e.development.run:443", CertSecretRef: &certC},
			},
			Env: map[string]string{
				"RAVEN_Cluster_TimeBeforeMovingToRehabInSec": "10",
			},

			ExternalAccessConfiguration: &ravendbv1.ExternalAccessConfiguration{
				Type:                            "ingress-controller",
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
