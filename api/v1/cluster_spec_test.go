package v1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func baseClusterForClusterSpecTest(name string) *RavenDBCluster {
	email := "user@example.com"
	certSecretRef := "cert-secret"
	return &RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: RavenDBClusterSpec{
			Image:                "ravendb/ravendb:latest",
			ImagePullPolicy:      "Always",
			Mode:                 "None",
			Email:                &email,
			LicenseSecretRef:     "license-secret",
			ClusterCertSecretRef: &certSecretRef,
			ClientCertSecretRef:  "client-cert",
			Domain:               "example.com",
			Nodes: []RavenDBNode{
				{
					Tag:                "A",
					PublicServerUrl:    "https://a.example.com",
					PublicServerUrlTcp: "tcp://a-tcp.example.com",
				},
			},
			StorageSpec: StorageSpec{
				Data: VolumeSpec{
					Size: "5Gi",
				},
			},
		},
	}
}

func TestImageValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid image",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "missing image",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Image = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.image", "should be at least 1 chars long"},
		},
	}
	runSpecValidationTest(t, baseClusterForClusterSpecTest, testCases)
}

func TestImagePullPolicyValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid pull policy Always",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "valid pull policy IfNotPresent",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ImagePullPolicy = "IfNotPresent"
			},
			ExpectError: false,
		},
		{
			Name: "invalid pull policy",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ImagePullPolicy = "Never"
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.imagePullPolicy", "Unsupported value"},
		},
		{
			Name: "empty value",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ImagePullPolicy = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.imagePullPolicy", "Unsupported value"},
		},
	}
	runSpecValidationTest(t, baseClusterForClusterSpecTest, testCases)
}

func TestModeValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid mode None",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "valid mode LetsEncrypt",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Mode = "LetsEncrypt"
			},
			ExpectError: false,
		},
		{
			Name: "invalid mode",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Mode = "SelfSigned"
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.mode", "Unsupported value"},
		},
		{
			Name: "empty mode value",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Mode = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.mode", "value: \"\""},
		},
	}
	runSpecValidationTest(t, baseClusterForClusterSpecTest, testCases)
}

func TestEmailValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid email",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "nil email (omitted)",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Email = nil
			},
			ExpectError: false,
		},
		{
			Name: "invalid email - no @",
			Modify: func(spec *RavenDBClusterSpec) {
				email := "admin.example.com"
				spec.Email = &email
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.email", "admin.example.com"},
		},
		{
			Name: "invalid email - no domain",
			Modify: func(spec *RavenDBClusterSpec) {
				email := "admin@"
				spec.Email = &email
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.email", "admin@"},
		},
	}

	runSpecValidationTest(t, baseClusterForClusterSpecTest, testCases)
}

func TestLicenseValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid license secret ref",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "missing license",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.LicenseSecretRef = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.licenseSecretRef"},
		},
	}
	runSpecValidationTest(t, baseClusterForClusterSpecTest, testCases)
}

func TestClusterCertSecretRefValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid cluster cert secret ref",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "nil cluster cert secret ref (omitted)",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ClusterCertSecretRef = nil
			},
			ExpectError: false,
		},
		{
			Name: "empty cluster cert secret ref",
			Modify: func(spec *RavenDBClusterSpec) {
				empty := ""
				spec.ClusterCertSecretRef = &empty
			},
			ExpectError: false,
		},
	}
	runSpecValidationTest(t, baseClusterForClusterSpecTest, testCases)
}

func TestDomainValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid domain",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "missing domain",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Domain = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.domain"},
		},
	}
	runSpecValidationTest(t, baseClusterForClusterSpecTest, testCases)
}

func TestEnvValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid env map",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Env = map[string]string{
					"RAVEN_ENV":     "value",
					"Raven_ANOTHER": "123",
					"MY_ENV_VAR":    "test",
				}
			},
			ExpectError: false,
		},
		{
			Name: "empty env map",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Env = map[string]string{}
			},
			ExpectError: false,
		},
		{
			Name: "nil env",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Env = nil
			},
			ExpectError: false,
		},
	}
	runSpecValidationTest(t, baseClusterForClusterSpecTest, testCases)
}

func TestClientCertSecretRefValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name:        "valid client cert secret ref",
			Modify:      func(spec *RavenDBClusterSpec) {},
			ExpectError: false,
		},
		{
			Name: "missing client cert secret ref",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ClientCertSecretRef = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.clientCertSecretRef", "should be at least 1 chars long"},
		},
	}
	runSpecValidationTest(t, baseClusterForClusterSpecTest, testCases)
}
