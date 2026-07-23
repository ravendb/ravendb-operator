package testutil

import (

	ravendbv1 "ravendb-operator/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


func BaseClusterLE(name string) *ravendbv1.RavenDBCluster {
	email := "user@ravendb.net"
	certA, certB, certC := "ravendb-certs-a", "ravendb-certs-b", "ravendb-certs-c"
	caCert := "ravendb-ca-cert"
	storageClass := "local-path"


	return &ravendbv1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},

		Spec: ravendbv1.RavenDBClusterSpec{
			Image:               "ravendb/ravendb:6.2.11-ubuntu.22.04-x64",
			ImagePullPolicy:     "IfNotPresent",
			Mode:                "LetsEncrypt",
			Email:               &email,
			LicenseSecretRef:    "ravendb-license",
			ClientCertSecretRef: "ravendb-client-cert",
			CACertSecretRef:     &caCert,

			Domain: "ravendb-operator-e2e.ravendb.run",

			Nodes: []ravendbv1.RavenDBNode{

				{Tag: "a", PublicServerUrl: "https://a.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://a-tcp.ravendb-operator-e2e.ravendb.run:443", CertSecretRef: &certA},
				{Tag: "b", PublicServerUrl: "https://b.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://b-tcp.ravendb-operator-e2e.ravendb.run:443", CertSecretRef: &certB},
				{Tag: "c", PublicServerUrl: "https://c.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://c-tcp.ravendb-operator-e2e.ravendb.run:443", CertSecretRef: &certC},
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

func BaseClusterLEDEF(name string) *ravendbv1.RavenDBCluster {
	email := "user@ravendb.net"
	certD, certE, certF := "ravendb-certs-d", "ravendb-certs-e", "ravendb-certs-f"
	caCert := "ravendb-ca-cert"
	storageClass := "local-path"

	return &ravendbv1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ravendbv1.RavenDBClusterSpec{
			Image:               "ravendb/ravendb:6.2.11-ubuntu.22.04-x64",
			ImagePullPolicy:     "IfNotPresent",
			Mode:                "LetsEncrypt",
			Email:               &email,
			LicenseSecretRef:    "ravendb-license",
			ClientCertSecretRef: "ravendb-client-cert",
			CACertSecretRef:     &caCert,
			Domain:              "ravendb-operator-e2e.ravendb.run",
			Nodes: []ravendbv1.RavenDBNode{
				{Tag: "d", PublicServerUrl: "https://d.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://d-tcp.ravendb-operator-e2e.ravendb.run:443", CertSecretRef: &certD},
				{Tag: "e", PublicServerUrl: "https://e.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://e-tcp.ravendb-operator-e2e.ravendb.run:443", CertSecretRef: &certE},
				{Tag: "f", PublicServerUrl: "https://f.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://f-tcp.ravendb-operator-e2e.ravendb.run:443", CertSecretRef: &certF},
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
