// /*
// Copyright 2025.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package v1alpha1_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"ravendb-operator/api/v1alpha1"
	"ravendb-operator/pkg/webhook/mutator"
	"ravendb-operator/pkg/webhook/validator"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func baseCluster(name string) *v1alpha1.RavenDBCluster {
	email := "me@example.com"
	cert := "cert"
	return &v1alpha1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: v1alpha1.RavenDBClusterSpec{
			Image:                "ravendb/ravendb:latest",
			ImagePullPolicy:      "IfNotPresent",
			Mode:                 "None",
			Email:                &email,
			LicenseSecretRef:     "license",
			Domain:               "example.com",
			ClusterCertSecretRef: &cert,
			Nodes: []v1alpha1.RavenDBNode{
				{
					Tag:                "A",
					PublicServerUrl:    "https://a.example.com",
					PublicServerUrlTcp: "tcp://a-tcp.example.com",
				},
			},
			StorageSpec: v1alpha1.StorageSpec{
				Data: v1alpha1.VolumeSpec{Size: "5Gi"},
			},
		},
	}
}

func baseClusterLetsEncrypt(name string) *v1alpha1.RavenDBCluster {
	email := "me@example.com"
	certa := "cert-a"
	certb := "cert-b"
	return &v1alpha1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "ravenedb",
		},
		Spec: v1alpha1.RavenDBClusterSpec{
			Image:            "ravendb/ravendb:latest",
			ImagePullPolicy:  "Always",
			Mode:             "LetsEncrypt",
			Email:            &email,
			LicenseSecretRef: "license",
			Domain:           "example.com",
			Nodes: []v1alpha1.RavenDBNode{
				{
					Tag:                "A",
					PublicServerUrl:    "https://a.example.com:443",
					PublicServerUrlTcp: "tcp://a-tcp.example.com:443",
					CertSecretRef:      &certa,
				},
				{
					Tag:                "B",
					PublicServerUrl:    "https://b.example.com",
					PublicServerUrlTcp: "tcp://b-tcp.example.com",
					CertSecretRef:      &certb,
				},
			},
			StorageSpec: v1alpha1.StorageSpec{
				Data: v1alpha1.VolumeSpec{Size: "5Gi"},
			},
		},
	}
}

func TestImageValidator(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects bad image", func(t *testing.T) {
		cluster := baseCluster("bad")
		cluster.Spec.Image = "thegoldenplatypus/ravendb"
		err := validator.RunCreate(ctx, cluster)
		require.Error(t, err)
		t.Logf("%v", err)
		require.Contains(t, err.Error(), "image must be from the 'ravendb/' registry namespace")
	})

	t.Run("accepts good image", func(t *testing.T) {
		cluster := baseCluster("good")
		err := validator.RunCreate(ctx, cluster)
		require.NoError(t, err)
	})
}

func TestPullPolicyMutator(t *testing.T) {

	t.Run("sets Always ipp for latest tag", func(t *testing.T) {
		cluster := baseCluster("latest-image")
		warnings, err := mutator.Run(cluster)
		require.NoError(t, err)
		require.Equal(t, "Always", cluster.Spec.ImagePullPolicy)
		require.Len(t, warnings, 1)
		require.Contains(t, warnings[0], "image \"ravendb/ravendb:latest\" uses ':latest'")
		require.Contains(t, warnings[0], "pull-policy-mutator")
		t.Logf("%v", warnings[0])

	})

	t.Run("keeps policy unset for specific version", func(t *testing.T) {
		cluster := baseCluster("versioned-image")
		cluster.Spec.Image = "ravendb/ravendb:6.0.0"
		warnings, err := mutator.Run(cluster)
		require.NoError(t, err)
		require.Empty(t, warnings)
	})
}

func TestGeneralValidatorValidateEmail(t *testing.T) {

	t.Run("reject missing email on LetsEncrypt mode", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("missing-email")
		cluster.Spec.Email = nil
		errs := validator.ValidateEmail(cluster.GetMode(), cluster.GetEmail())
		require.NotEmpty(t, errs)
		t.Logf("%v", errs)
		require.Contains(t, errs[0], "spec.email is required when mode is LetsEncrypt")

	})

	t.Run("accept email on LetsEncrypt mode", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("existing-email")
		errs := validator.ValidateEmail(cluster.GetMode(), cluster.GetEmail())
		require.Empty(t, errs)
	})

	t.Run("reject email on None mode", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("existing-email-none-mode")
		cluster.Spec.Mode = "None"
		errs := validator.ValidateEmail(cluster.GetMode(), cluster.GetEmail())
		require.NotEmpty(t, errs)
		t.Logf("%v", errs)
		require.Contains(t, errs[0], "spec.email must not be set when mode is None")

	})

	t.Run("accept missing email on None mode", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("missing-email-on-none-mode")
		cluster.Spec.Mode = "None"
		cluster.Spec.Email = nil
		errs := validator.ValidateEmail(cluster.GetMode(), cluster.GetEmail())
		require.Empty(t, errs)
	})
}

func TestGeneralValidatorValidateLicenseSecret(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().
		WithObjects(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "license",
					Namespace: "ravendb",
				},
				Data: map[string][]byte{
					"license.json": []byte("{}"),
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-json-key-license",
					Namespace: "ravendb",
				},
				Data: map[string][]byte{
					"license.txt": []byte("{}"),
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-license-multi-keys",
					Namespace: "ravendb",
				},
				Data: map[string][]byte{
					"one.json": []byte("{}"),
					"two.json": []byte("{}"),
				},
			},
		).Build()

	v := validator.NewGeneralValidator(client)

	t.Run("valid license secret", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("valid-license")
		errs := validator.ValidateLicenseSecret(v, ctx, cluster.GetLicenseSecretRef())
		require.Empty(t, errs)
	})

	t.Run("licnese secret missing", func(t *testing.T) {
		errs := validator.ValidateLicenseSecret(v, ctx, "non-existing-secret")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.licenseSecretRef: secret 'non-existing-secret' not found")
	})

	t.Run("license secret with non-json key", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("invalid-license")
		cluster.Spec.LicenseSecretRef = "non-json-key-license"
		errs := validator.ValidateLicenseSecret(v, ctx, cluster.GetLicenseSecretRef())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.licenseSecretRef: secret 'non-json-key-license' must contain a file ending with '.json'")
	})

	t.Run("license secret with multiple keys", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("invalid-license-multi-keys")
		cluster.Spec.LicenseSecretRef = "invalid-license-multi-keys"
		errs := validator.ValidateLicenseSecret(v, ctx, cluster.GetLicenseSecretRef())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.licenseSecretRef: secret 'invalid-license-multi-keys' must contain exactly one '.json' file")
	})
}

func TestGeneralValidatorValidateClusterCertSecret(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().
		WithObjects(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-cluster-cert",
					Namespace: "ravendb",
				},
				Data: map[string][]byte{
					"cluster.pfx": []byte("fake"),
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-pfx-cluster-cert",
					Namespace: "ravendb",
				},
				Data: map[string][]byte{
					"cert.pem": []byte("fake"),
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-key-cluster-cert",
					Namespace: "ravendb",
				},
				Data: map[string][]byte{
					"one.pfx": []byte("a"),
					"two.pfx": []byte("b"),
				},
			},
		).Build()

	v := validator.NewGeneralValidator(client)

	t.Run("reject clusterCert when mode is LetsEncrypt", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("invalid-license-multi-keys")
		cert := "valid-cluster-cert"
		cluster.Spec.ClusterCertSecretRef = &cert
		errs := validator.ValidateClusterCertSecret(v, ctx, cluster.GetMode(), cluster.GetClusterCertsSecretRef())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.clusterCertSecretRef must not be set when mode is LetsEncrypt")
	})

	t.Run("require clusterCert when mode is None and cert is missing", func(t *testing.T) {
		cluster := baseCluster("missing-cert")
		cluster.Spec.ClusterCertSecretRef = nil
		cluster.Spec.Mode = "None"
		errs := validator.ValidateClusterCertSecret(v, ctx, cluster.GetMode(), cluster.GetClusterCertsSecretRef())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.clusterCertSecretRef is required when mode is None")
	})

	t.Run("reject missing secret", func(t *testing.T) {
		cluster := baseCluster("missing-secret")
		secret := "non-existent"
		cluster.Spec.ClusterCertSecretRef = &secret
		cluster.Spec.Mode = "None"
		errs := validator.ValidateClusterCertSecret(v, ctx, cluster.GetMode(), cluster.GetClusterCertsSecretRef())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.clusterCertSecretRef: secret 'non-existent' not found")
	})

	t.Run("reject secret with non-pfx key", func(t *testing.T) {
		cluster := baseCluster("non-pfx")
		secret := "non-pfx-cluster-cert"
		cluster.Spec.ClusterCertSecretRef = &secret
		errs := validator.ValidateClusterCertSecret(v, ctx, cluster.GetMode(), cluster.GetClusterCertsSecretRef())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.clusterCertSecretRef: secret 'non-pfx-cluster-cert' must contain a file ending with '.pfx")
	})

	t.Run("reject secret with multiple keys", func(t *testing.T) {
		cluster := baseCluster("multi-key")
		secret := "multi-key-cluster-cert"
		cluster.Spec.ClusterCertSecretRef = &secret
		errs := validator.ValidateClusterCertSecret(v, ctx, cluster.GetMode(), cluster.GetClusterCertsSecretRef())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.clusterCertSecretRef: secret 'multi-key-cluster-cert' must contain exactly one '.pfx' file")
	})

	t.Run("accept valid cluster cert", func(t *testing.T) {
		cluster := baseCluster("valid-cert")
		secret := "valid-cluster-cert"
		cluster.Spec.ClusterCertSecretRef = &secret
		errs := validator.ValidateClusterCertSecret(v, ctx, cluster.GetMode(), cluster.GetClusterCertsSecretRef())
		require.Empty(t, errs)
	})
}

func TestGeneralValidatorValidateDomain(t *testing.T) {
	t.Run("reject domain with underscore", func(t *testing.T) {
		cluster := baseCluster("bad-underscore")
		cluster.Spec.Domain = "bad_domain.com"
		errs := validator.ValidateDomain(cluster.GetDomain())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.domain 'bad_domain.com' must be a valid FQDN")
	})

	t.Run("reject domain with localhost", func(t *testing.T) {
		cluster := baseCluster("bad-localhost")
		cluster.Spec.Domain = "localhost"
		errs := validator.ValidateDomain(cluster.GetDomain())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.domain 'localhost' must be a valid FQDN")
	})

	t.Run("reject domain that is an IP", func(t *testing.T) {
		cluster := baseCluster("bad-ip")
		cluster.Spec.Domain = "127.0.0.1"
		errs := validator.ValidateDomain(cluster.GetDomain())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.domain '127.0.0.1' must be a valid FQDN")
	})

	t.Run("accept valid domain", func(t *testing.T) {
		cluster := baseCluster("valid-domain")
		cluster.Spec.Domain = "example.com"
		errs := validator.ValidateDomain(cluster.GetDomain())
		require.Empty(t, errs)
	})
	t.Run("accept valid domain - local", func(t *testing.T) {
		cluster := baseCluster("valid-domain")
		cluster.Spec.Domain = "local"
		errs := validator.ValidateDomain(cluster.GetDomain())
		require.Empty(t, errs)
	})
}

func TestGeneralValidatorValidateEnv(t *testing.T) {
	t.Run("accept env var without RAVEN_ prefix", func(t *testing.T) {
		cluster := baseCluster("no-prefix")
		cluster.Spec.Env = map[string]string{
			"DEBUG": "true",
		}
		errs := validator.ValidateEnv(cluster.GetEnv())
		require.Empty(t, errs)
	})

	t.Run("accepts valid env vars", func(t *testing.T) {
		cluster := baseCluster("valid-env")
		cluster.Spec.Env = map[string]string{
			"RAVEN_Setup_Mode": "None",
			"RAVEN_Port":       "8080",
		}
		errs := validator.ValidateEnv(cluster.GetEnv())
		require.Empty(t, errs)
	})
}

func TestNodeValidatorValidateNodesNotEmpty(t *testing.T) {
	t.Run("rejects empty nodes list", func(t *testing.T) {
		cluster := baseCluster("no-nodes")
		cluster.Spec.Nodes = []v1alpha1.RavenDBNode{}
		errs := validator.ValidateNodesNotEmpty(cluster.GetNodeTags())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.nodes must contain at least one node")
	})

	t.Run("accepts non-empty nodes list", func(t *testing.T) {
		cluster := baseCluster("has-nodes")
		errs := validator.ValidateNodesNotEmpty(cluster.GetNodeTags())
		require.Empty(t, errs)
	})
}

func TestNodeValidatorValidateUniqueTags(t *testing.T) {
	t.Run("rejects duplicate tags", func(t *testing.T) {
		cluster := baseCluster("duplciate-tags")
		cluster.Spec.Nodes = []v1alpha1.RavenDBNode{
			{Tag: "A"},
			{Tag: "A"},
		}
		errs := validator.ValidateUniqueTags(cluster.GetNodeTags())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.nodes: duplicate tag 'A'")
	})

	t.Run("accepts unique tags", func(t *testing.T) {
		cluster := baseCluster("unique-tags")
		cluster.Spec.Nodes = []v1alpha1.RavenDBNode{
			{Tag: "A"},
			{Tag: "B"},
		}
		errs := validator.ValidateUniqueTags(cluster.GetNodeTags())
		require.Empty(t, errs)
	})
}

func TestNodeValidatorValidateUniqueUrls(t *testing.T) {
	t.Run("rejects duplicate publicServerUrl", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("duplicate-public")
		cluster.Spec.Nodes[1].PublicServerUrl = cluster.Spec.Nodes[0].PublicServerUrl

		errs := validator.ValidateUniqueUrls(cluster.GetNodePublicUrls(), cluster.GetNodeTcpUrls())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.nodes[1].publicServerUrl duplicates URL already used in spec.nodes[0].publicServerUrl")
	})

	t.Run("rejects duplicate publicServerUrlTcp", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("duplciate-tcp")
		cluster.Spec.Nodes[1].PublicServerUrlTcp = cluster.Spec.Nodes[0].PublicServerUrlTcp

		errs := validator.ValidateUniqueUrls(cluster.GetNodePublicUrls(), cluster.GetNodeTcpUrls())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.nodes[1].publicServerUrlTcp duplicates URL already used in spec.nodes[0].publicServerUrlTcp")
	})

	t.Run("rejects duplicate between public and tcp", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("duplciate-mixed")
		cluster.Spec.Nodes[1].PublicServerUrlTcp = cluster.Spec.Nodes[0].PublicServerUrl

		errs := validator.ValidateUniqueUrls(cluster.GetNodePublicUrls(), cluster.GetNodeTcpUrls())
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.nodes[1].publicServerUrlTcp duplicates URL already used in spec.nodes[0].publicServerUrl")
	})

	t.Run("accepts unique URLs", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("unique")
		errs := validator.ValidateUniqueUrls(cluster.GetNodePublicUrls(), cluster.GetNodeTcpUrls())
		require.Empty(t, errs)
	})
}

func TestNodeValidatorValidatePortsConsistency(t *testing.T) {
	t.Run("rejects when public and tcp ports differ for a node", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("port-mismatch")
		cluster.Spec.Nodes[0].PublicServerUrl = "https://a.example.com:443"
		cluster.Spec.Nodes[0].PublicServerUrlTcp = "tcp://a-tcp.example.com:38888"

		errs := validator.ValidatePortsConsistency(cluster.GetNodePublicUrls(), cluster.GetNodeTcpUrls(), "ingress-controller")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.nodes: publicServerUrl and publicServerUrlTcp ports must match")
	})

	t.Run("rejects when ports differ across nodes", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("node-port-inconsistency")
		cluster.Spec.Nodes[0].PublicServerUrl = "https://a.example.com:443"
		cluster.Spec.Nodes[0].PublicServerUrlTcp = "tcp://a-tcp.example.com:443"
		cluster.Spec.Nodes[1].PublicServerUrl = "https://b.example.com:1234"
		cluster.Spec.Nodes[1].PublicServerUrlTcp = "tcp://b-tcp.example.com:1234"

		errs := validator.ValidatePortsConsistency(cluster.GetNodePublicUrls(), cluster.GetNodeTcpUrls(), "ingress-controller")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "spec.nodes: ports must be consistent across all nodes")
	})

	t.Run("accepts when all ports match and are consistent", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("consistent")
		cluster.Spec.Nodes[0].PublicServerUrl = "https://a.example.com:443"
		cluster.Spec.Nodes[0].PublicServerUrlTcp = "tcp://a-tcp.example.com:443"
		cluster.Spec.Nodes[1].PublicServerUrl = "https://b.example.com:443"
		cluster.Spec.Nodes[1].PublicServerUrlTcp = "tcp://b-tcp.example.com:443"

		errs := validator.ValidatePortsConsistency(cluster.GetNodePublicUrls(), cluster.GetNodeTcpUrls(), "ingress-controller")
		require.Empty(t, errs)
	})
}

func TestValidateNodeUrl(t *testing.T) {
	cluster := baseClusterLetsEncrypt("node-url")
	node := cluster.Spec.Nodes[0]
	tag := node.Tag
	domain := cluster.Spec.Domain

	t.Run("accept valid url", func(t *testing.T) {
		validUrl := node.PublicServerUrl
		errs := validator.ValidateNodeUrl(tag, validUrl, domain, "https", "publicServerUrl", "a.")
		require.Empty(t, errs)
	})

	t.Run("reject invalid scheme", func(t *testing.T) {
		badUrl := strings.Replace(node.PublicServerUrl, "https://", "http://", 1)
		errs := validator.ValidateNodeUrl(tag, badUrl, domain, "https", "publicServerUrl", "a.")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "scheme must be 'https'")
	})

	t.Run("reject wrong prefix", func(t *testing.T) {
		badUrl := strings.Replace(node.PublicServerUrl, "a.", "wrong.", 1)
		errs := validator.ValidateNodeUrl(tag, badUrl, domain, "https", "publicServerUrl", "a.")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "hostname must start with 'a.'")
	})

	t.Run("reject non-subdomain", func(t *testing.T) {
		badUrl := strings.Replace(node.PublicServerUrl, domain, "other.com", 1)
		errs := validator.ValidateNodeUrl(tag, badUrl, domain, "https", "publicServerUrl", "a.")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "hostname must be subdomain of '"+domain+"'")
	})

	t.Run("reject missing port", func(t *testing.T) {
		u, _ := url.Parse(node.PublicServerUrl)
		u.Host = u.Hostname()
		badUrl := u.String()

		errs := validator.ValidateNodeUrl(tag, badUrl, domain, "https", "publicServerUrl", "a.")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "must include a port")
	})

	t.Run("reject URL with path", func(t *testing.T) {
		badUrl := node.PublicServerUrl + "/path"
		errs := validator.ValidateNodeUrl(tag, badUrl, domain, "https", "publicServerUrl", "a.")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "must not contain path")
	})

	t.Run("reject URL with query", func(t *testing.T) {
		badUrl := node.PublicServerUrl + "?q=x"
		errs := validator.ValidateNodeUrl(tag, badUrl, domain, "https", "publicServerUrl", "a.")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "must not contain query")
	})

	t.Run("reject URL with fragment", func(t *testing.T) {
		badUrl := node.PublicServerUrl + "#frag"
		errs := validator.ValidateNodeUrl(tag, badUrl, domain, "https", "publicServerUrl", "a.")
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "must not contain fragment")
	})
}

func TestValidateNodeCertSecret(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().
		WithObjects(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-cert",
					Namespace: "ravendb",
				},
				Data: map[string][]byte{
					"node.pfx": []byte("fake"),
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-pfx-cert",
					Namespace: "ravendb",
				},
				Data: map[string][]byte{
					"cert.pem": []byte("fake"),
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-key-cert",
					Namespace: "ravendb",
				},
				Data: map[string][]byte{
					"one.pfx": []byte("a"),
					"two.pfx": []byte("b"),
				},
			},
		).Build()

	v := validator.NewNodeValidator(client)

	t.Run("require cert when mode is LetsEncrypt and missing", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("missing-cert")
		cluster.Spec.Nodes[0].CertSecretRef = nil
		tag := cluster.Spec.Nodes[0].Tag
		certRef := ""
		errs := validator.ValidateNodeCertSecret(ctx, v, cluster.GetMode(), tag, certRef)
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "is required when mode is LetsEncrypt")
	})

	t.Run("reject cert when mode is None", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("reject-cert")
		cluster.Spec.Mode = "None"
		tag := cluster.Spec.Nodes[0].Tag
		certRefPtr := cluster.GetNodeCertSecretRefs()[0]
		certRef := ""
		if certRefPtr != nil {
			certRef = *certRefPtr
		}
		errs := validator.ValidateNodeCertSecret(ctx, v, cluster.GetMode(), tag, certRef)
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "must not be set when mode is None")
	})

	t.Run("reject missing secret", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("missing-secret")
		secret := "non-existent"
		cluster.Spec.Nodes[0].CertSecretRef = &secret
		tag := cluster.Spec.Nodes[0].Tag
		certRef := *cluster.Spec.Nodes[0].CertSecretRef
		errs := validator.ValidateNodeCertSecret(ctx, v, cluster.GetMode(), tag, certRef)
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "secret 'non-existent' not found")
	})

	t.Run("reject non-pfx secret", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("non-pfx")
		secret := "non-pfx-cert"
		cluster.Spec.Nodes[0].CertSecretRef = &secret
		tag := cluster.Spec.Nodes[0].Tag
		certRef := *cluster.Spec.Nodes[0].CertSecretRef
		errs := validator.ValidateNodeCertSecret(ctx, v, cluster.GetMode(), tag, certRef)
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "file 'cert.pem' must end with .pfx")
	})

	t.Run("reject multi-key secret", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("multi-key")
		secret := "multi-key-cert"
		cluster.Spec.Nodes[0].CertSecretRef = &secret
		tag := cluster.Spec.Nodes[0].Tag
		certRef := *cluster.Spec.Nodes[0].CertSecretRef
		errs := validator.ValidateNodeCertSecret(ctx, v, cluster.GetMode(), tag, certRef)
		require.NotEmpty(t, errs)
		require.Contains(t, errs[0], "must contain exactly one .pfx file")
	})

	t.Run("accept valid cert", func(t *testing.T) {
		cluster := baseClusterLetsEncrypt("valid")
		secret := "valid-cert"
		cluster.Spec.Nodes[0].CertSecretRef = &secret
		tag := cluster.Spec.Nodes[0].Tag
		certRef := *cluster.Spec.Nodes[0].CertSecretRef
		errs := validator.ValidateNodeCertSecret(ctx, v, cluster.GetMode(), tag, certRef)
		require.Empty(t, errs)
	})
}

func TestEAValidator(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()
	v := validator.NewEaValidator(client)

	t.Run("accepts valid aws config", func(t *testing.T) {
		cluster := baseCluster("valid-aws")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("aws-nlb"),
			AWSExternalAccess: &v1alpha1.AWSExternalAccessContext{
				NodeMappings: []v1alpha1.AWSNodeMapping{
					{
						Tag:             "A",
						EIPAllocationId: "eipalloc-0123456789abcdef0",
						SubnetId:        "subnet-abcdef1234567890",
					},
				},
			},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.NoError(t, err)
	})

	t.Run("accepts valid azure config", func(t *testing.T) {
		cluster := baseCluster("valid-azure")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("azure-lb"),
			AzureExternalAccess: &v1alpha1.AzureExternalAccessContext{
				NodeMappings: []v1alpha1.AzureNodeMapping{
					{
						Tag: "A",
						IP:  "1.2.3.4",
					},
				},
			},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.NoError(t, err)
	})

	t.Run("accepts valid ingress-controller config (nginx)", func(t *testing.T) {
		cluster := baseCluster("valid-ingress")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("ingress-controller"),
			IngressControllerExternalAccess: &v1alpha1.IngressControllerContext{
				IngressClassName: "nginx",
			},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.NoError(t, err)
	})

	t.Run("accepts valid ingress-controller config (haproxy)", func(t *testing.T) {
		cluster := baseCluster("valid-ingress-haproxy")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("ingress-controller"),
			IngressControllerExternalAccess: &v1alpha1.IngressControllerContext{
				IngressClassName: "haproxy",
			},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.NoError(t, err)
	})

	t.Run("accepts valid ingress-controller config (traefik)", func(t *testing.T) {
		cluster := baseCluster("valid-ingress-traefik")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("ingress-controller"),
			IngressControllerExternalAccess: &v1alpha1.IngressControllerContext{
				IngressClassName: "traefik",
			},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.NoError(t, err)
	})

	t.Run("rejects ingress with ssl-passthrough=false (nginx)", func(t *testing.T) {
		cluster := baseCluster("ssl-false-nginx")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("ingress-controller"),
			IngressControllerExternalAccess: &v1alpha1.IngressControllerContext{
				IngressClassName: "nginx",
				AdditionalAnnotations: map[string]string{
					"nginx.ingress.kubernetes.io/ssl-passthrough": "false",
				},
			},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.Error(t, err)
		require.Contains(t, err.Error(), `must not contain 'nginx.ingress.kubernetes.io/ssl-passthrough: "false"'`)
	})

	t.Run("rejects ingress with ssl-passthrough=false (haproxy)", func(t *testing.T) {
		cluster := baseCluster("ssl-false-haproxy")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("ingress-controller"),
			IngressControllerExternalAccess: &v1alpha1.IngressControllerContext{
				IngressClassName: "haproxy",
				AdditionalAnnotations: map[string]string{
					"haproxy.org/ssl-passthrough": "false",
				},
			},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.Error(t, err)
		require.Contains(t, err.Error(), `must not contain 'haproxy.org/ssl-passthrough: "false"'`)
	})

	t.Run("rejects missing context for ingress-controller", func(t *testing.T) {
		cluster := baseCluster("missing-ingress")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("ingress-controller"),
		}
		err := v.ValidateCreate(ctx, cluster)
		require.Error(t, err)
		require.Contains(t, err.Error(), "spec.externalAccessConfiguration.ingressControllerContext is required")
	})

	t.Run("rejects missing context for aws", func(t *testing.T) {
		cluster := baseCluster("missing-aws")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("aws-nlb"),
		}
		err := v.ValidateCreate(ctx, cluster)
		require.Error(t, err)
		require.Contains(t, err.Error(), "spec.externalAccessConfiguration.awsExternalAccessContext is required")
	})

	t.Run("rejects missing context for azure", func(t *testing.T) {
		cluster := baseCluster("missing-azure")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("azure-lb"),
		}
		err := v.ValidateCreate(ctx, cluster)
		require.Error(t, err)
		require.Contains(t, err.Error(), "spec.externalAccessConfiguration.azureExternalAccessContext is required")
	})

	t.Run("rejects conflicting contexts for ingress-controller", func(t *testing.T) {
		cluster := baseCluster("conflict-ingress")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: v1alpha1.ExternalAccessType("ingress-controller"),
			IngressControllerExternalAccess: &v1alpha1.IngressControllerContext{
				IngressClassName: "nginx",
			},
			AWSExternalAccess: &v1alpha1.AWSExternalAccessContext{},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.Error(t, err)
		require.Contains(t, err.Error(), "must not be set when type is 'ingress-controller'")
	})

	t.Run("rejects conflicting contexts for aws", func(t *testing.T) {
		cluster := baseCluster("conflict-aws")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type:              v1alpha1.ExternalAccessType("aws-nlb"),
			AWSExternalAccess: &v1alpha1.AWSExternalAccessContext{},
			IngressControllerExternalAccess: &v1alpha1.IngressControllerContext{
				IngressClassName: "nginx",
			},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.Error(t, err)
		require.Contains(t, err.Error(), "must not be set when type is 'aws-nlb'")
	})

	t.Run("rejects conflicting contexts for azure", func(t *testing.T) {
		cluster := baseCluster("conflict-azure")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type:                v1alpha1.ExternalAccessType("azure-lb"),
			AzureExternalAccess: &v1alpha1.AzureExternalAccessContext{},
			AWSExternalAccess:   &v1alpha1.AWSExternalAccessContext{},
		}
		err := v.ValidateCreate(ctx, cluster)
		require.Error(t, err)
		require.Contains(t, err.Error(), "must not be set when type is 'azure-lb'")
	})

	t.Run("rejects unknown external access type", func(t *testing.T) {
		cluster := baseCluster("invalid-type")
		cluster.Spec.ExternalAccessConfiguration = &v1alpha1.ExternalAccessConfiguration{
			Type: "bagira",
		}
		err := v.ValidateCreate(ctx, cluster)
		require.Error(t, err)
		require.Contains(t, err.Error(), "spec.externalAccessConfiguration.type has invalid value: 'bagira'")
	})
}

func TestStorageValidatorValidateVolumeSpec(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientBuilder().Build()
	v := validator.NewStorageValidator(client)

	t.Run("calls ValidateRWX when storageClassName is set", func(t *testing.T) {
		sc := "standard"
		am := []string{"ReadWriteMany"}
		errs := v.ValidateVolumeSpec(ctx, "spec.storage.data", &sc, am, nil)
		// ValidateRWX is not implemented yet  only test that no storageClass warning appears
		require.NotContains(t, strings.Join(errs, "\n"), "storageClassName is not set")
	})

	t.Run("accept if VAC is nil", func(t *testing.T) {
		sc := "standard"
		am := []string{"ReadWriteOnce"}
		errs := v.ValidateVolumeSpec(ctx, "spec.storage.data", &sc, am, nil)
		require.NotContains(t, strings.Join(errs, ""), "volumeAttributesClassName")
	})

	t.Run("fails if VAC is invalid", func(t *testing.T) {
		sc := "standard"
		am := []string{"ReadWriteOnce"}
		vac := "non-existent"

		errs := v.ValidateVolumeSpec(ctx, "spec.storage.data", &sc, am, &vac)
		require.Len(t, errs, 1)
		require.Contains(t, errs[0], "volumeAttributesClassName 'non-existent' does not reference a valid VolumeAttributesClass")
	})
}

func TestValidateAdditionalVolumes(t *testing.T) {
	path := "spec.storage.additionalVolumes"

	t.Run("rejects duplicate names", func(t *testing.T) {
		names := []string{"data", "data"}
		mounts := []*string{ptr("/mnt/a"), ptr("/mnt/b")}
		subpaths := []*string{nil, nil}
		sources := []map[string]bool{
			{"configMap": true},
			{"configMap": true},
		}

		errs := validator.ValidateAdditionalVolumes(path, names, mounts, subpaths, sources)
		require.Len(t, errs, 1)
		require.Contains(t, errs[0], "name must be unique")
	})

	t.Run("rejects non-absolute mountPath", func(t *testing.T) {
		names := []string{"a"}
		mounts := []*string{ptr("mnt/relative")}
		subpaths := []*string{nil}
		sources := []map[string]bool{{"configMap": true}}

		errs := validator.ValidateAdditionalVolumes(path, names, mounts, subpaths, sources)
		require.Len(t, errs, 1)
		require.Contains(t, errs[0], "must be an absolute path")
	})

	t.Run("rejects subPath with slashes", func(t *testing.T) {
		names := []string{"a"}
		mounts := []*string{ptr("/mnt/data")}
		subpaths := []*string{ptr("dir/file.txt")}
		sources := []map[string]bool{{"configMap": true}}

		errs := validator.ValidateAdditionalVolumes(path, names, mounts, subpaths, sources)
		require.Len(t, errs, 1)
		require.Contains(t, errs[0], "subPath must be a file name only")
	})

	t.Run("rejects missing volume source", func(t *testing.T) {
		names := []string{"a"}
		mounts := []*string{ptr("/mnt/data")}
		subpaths := []*string{nil}
		sources := []map[string]bool{{}}

		errs := validator.ValidateAdditionalVolumes(path, names, mounts, subpaths, sources)
		require.Len(t, errs, 1)
		require.Contains(t, errs[0], "must have exactly one source")
	})

	t.Run("rejects multiple volume sources", func(t *testing.T) {
		names := []string{"a"}
		mounts := []*string{ptr("/mnt/data")}
		subpaths := []*string{nil}
		sources := []map[string]bool{{
			"configMap": true,
			"secret":    true,
		}}

		errs := validator.ValidateAdditionalVolumes(path, names, mounts, subpaths, sources)
		require.Len(t, errs, 1)
		require.Contains(t, errs[0], "must have exactly one source")
	})

	t.Run("rejects invalid volume source key", func(t *testing.T) {
		names := []string{"a"}
		mounts := []*string{ptr("/mnt/data")}
		subpaths := []*string{nil}
		sources := []map[string]bool{{
			"badType": true,
		}}

		errs := validator.ValidateAdditionalVolumes(path, names, mounts, subpaths, sources)
		require.Len(t, errs, 1)
		require.Contains(t, errs[0], "contains invalid type: 'badType'")
	})

	t.Run("accepts fully valid volume", func(t *testing.T) {
		names := []string{"data"}
		mounts := []*string{ptr("/mnt/data")}
		subpaths := []*string{ptr("logs.txt")}
		sources := []map[string]bool{{
			"secret": true,
		}}

		errs := validator.ValidateAdditionalVolumes(path, names, mounts, subpaths, sources)
		require.Empty(t, errs)
	})
}

func ptr(s string) *string { return &s }
