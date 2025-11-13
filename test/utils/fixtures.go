package testutil

import (
	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func BaseClusterLE(name string) *ravendbv1alpha1.RavenDBCluster {
	email := "omer.ratsaby@ravendb.net"
	certA, certB, certC := "ravendb-certs-a", "ravendb-certs-b", "ravendb-certs-c"
	storageClass := "local-path"

	return &ravendbv1alpha1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ravendbv1alpha1.RavenDBClusterSpec{
			Image:               "ravendb/ravendb:6.2.9-ubuntu.22.04-x64",
			ImagePullPolicy:     "IfNotPresent",
			Mode:                "LetsEncrypt",
			Email:               &email,
			LicenseSecretRef:    "ravendb-license",
			ClientCertSecretRef: "ravendb-client-cert",
			Domain:              "ravendb-operator-e2e.ravendb.run",
			Nodes: []ravendbv1alpha1.RavenDBNode{
				{Tag: "a", PublicServerUrl: "https://a.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://a-tcp.ravendb-operator-e2e.ravendb.run:443", CertSecretRef: &certA},
				{Tag: "b", PublicServerUrl: "https://b.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://b-tcp.ravendb-operator-e2e.ravendb.run:443", CertSecretRef: &certB},
				{Tag: "c", PublicServerUrl: "https://c.ravendb-operator-e2e.ravendb.run:443", PublicServerUrlTcp: "tcp://c-tcp.ravendb-operator-e2e.ravendb.run:443", CertSecretRef: &certC},
			},
			ExternalAccessConfiguration: &ravendbv1alpha1.ExternalAccessConfiguration{
				Type:                            "ingress-controller",
				IngressControllerExternalAccess: &ravendbv1alpha1.IngressControllerContext{IngressClassName: "nginx"},
			},
			StorageSpec: ravendbv1alpha1.StorageSpec{
				Data: ravendbv1alpha1.VolumeSpec{
					Size:             "10Gi",
					StorageClassName: &storageClass,
				},
			},
		},
	}
}
