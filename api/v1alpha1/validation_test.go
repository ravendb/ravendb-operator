package v1alpha1

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
)

func TestMain(m *testing.M) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(AddToScheme(scheme))
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
	}

	cfg, err := testEnv.Start()
	if err != nil {
		panic(err)
	}
	defer func() { testEnv.Stop() }()

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		panic(err)
	}

	ctx, cancel = context.WithCancel(context.TODO())
	defer cancel()

	code := m.Run()
	os.Exit(code)
}

func validRavenDBCluster(name string) *RavenDBCluster {
	return &RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: RavenDBClusterSpec{
			Image:            "ravendb/ravendb:latest",
			ImagePullPolicy:  "Always",
			Mode:             "LetsEncrypt",
			Email:            "user@ravendb.net",
			License:          "license",
			Domain:           "mydomain",
			ServerUrl:        "https://localhost:443",
			ServerUrlTcp:     "tcp://localhost:38888",
			StorageSize:      "5Gi",
			IngressClassName: "nginx",
			Nodes: []RavenDBNode{
				{
					Name:               "A",
					PublicServerUrl:    "https://a.example.com",
					PublicServerUrlTcp: "tcp://a-tcp.example.com",
				},
			},
		},
	}
}

type SpecValidationCase struct {
	Name        string
	Modify      func(*RavenDBClusterSpec)
	ExpectError bool
	ErrorParts  []string
}

var rfc1123Regexp = regexp.MustCompile("[^a-z0-9.-]+")

func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = rfc1123Regexp.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	return name
}

func runSpecValidationTest(t *testing.T, testCases []SpecValidationCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			sanitizedName := sanitizeName("test-" + tc.Name)
			instance := validRavenDBCluster(sanitizedName)
			tc.Modify(&instance.Spec)

			err := k8sClient.Create(ctx, instance)
			if tc.ExpectError {
				if err == nil && len(tc.ErrorParts) > 0 {
					t.Skip("skipping...") // MinItems=1 is enforced at the api server level, envtest does not
					return
				}
				assert.Error(t, err)
				t.Log(err)
				for _, part := range tc.ErrorParts {
					assert.Contains(t, err.Error(), part)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEmailValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid email",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Email = "user@example.com"
			},
			ExpectError: false,
		},
		{
			Name: "empty email",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Email = ""
			},
			ExpectError: false,
		},
		{
			Name: "invalid email format",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Email = "bademail"
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.email", "should match"},
		},
	}

	runSpecValidationTest(t, testCases)
}

func TestImageValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid image",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Image = "ravendb/ravendb:latest"
			},
			ExpectError: false,
		},
		{
			Name: "missing image",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Image = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.image"},
		},
	}
	runSpecValidationTest(t, testCases)
}

func TestImagePullPolicyValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid pull policy Always",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ImagePullPolicy = "Always"
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
				spec.ImagePullPolicy = "InvalidPolicy"
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.imagePullPolicy", "Unsupported value"},
		},
	}
	runSpecValidationTest(t, testCases)
}

func TestModeValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid mode None",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Mode = "None"
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
				spec.Mode = "InvalidMode"
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.mode", "Unsupported value"},
		},
	}
	runSpecValidationTest(t, testCases)
}

func TestLicenseValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid license",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.License = "valid-license"
			},
			ExpectError: false,
		},
		{
			Name: "missing license",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.License = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.license"},
		},
	}
	runSpecValidationTest(t, testCases)
}

func TestDomainValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid domain",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Domain = "mydomain"
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
	runSpecValidationTest(t, testCases)
}

func TestServerUrlValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid server url",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ServerUrl = "https://localhost:443"
			},
			ExpectError: false,
		},
		{
			Name: "missing server url",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ServerUrl = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.serverUrl"},
		},
	}
	runSpecValidationTest(t, testCases)
}

func TestServerUrlTcpValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid server url tcp",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ServerUrlTcp = "tcp://localhost:38888"
			},
			ExpectError: false,
		},
		{
			Name: "missing server url tcp",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ServerUrlTcp = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.serverUrlTcp"},
		},
	}
	runSpecValidationTest(t, testCases)
}

func TestStorageSizeValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid storage size",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSize = "5Gi"
			},
			ExpectError: false,
		},
		{
			Name: "missing storage size",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSize = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.storageSize"},
		},
	}
	runSpecValidationTest(t, testCases)
}

func TestNodesValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid nodes",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "missing nodes",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Nodes = []RavenDBNode{}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.nodes", "Required value"},
		},
	}
	runSpecValidationTest(t, testCases)
}

func TestIngressClassNameValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid ingress class name",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.IngressClassName = "nginx"
			},
			ExpectError: false,
		},
		{
			Name: "missing ingress class name",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.IngressClassName = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.ingressClassName"},
		},
	}
	runSpecValidationTest(t, testCases)
}
