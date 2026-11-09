package samples_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	v1 "ravendb-operator/api/v1"

	"sigs.k8s.io/yaml"
)

// Strict-unmarshal config/samples/* against api/v1. Samples ride into the CSV's
// alm-examples; a renamed CRD field would silently ship a broken "Try sample CR"
// form. Own package keeps it envtest-free so prepare-release.yml can gate on it.
func TestConfigSamplesValid(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	pattern := filepath.Join(repoRoot, "config/samples/ravendb_v1_*.yaml")

	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatalf("no samples matched %q", pattern)
	}
	for _, p := range matches {
		t.Run(filepath.Base(p), func(t *testing.T) {
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			var typeMeta struct{ Kind string }
			if err := yaml.Unmarshal(data, &typeMeta); err != nil {
				t.Fatal(err)
			}
			target, ok := map[string]any{
				"RavenDBCluster":           &v1.RavenDBCluster{},
				"RavenDBDatabase":          &v1.RavenDBDatabase{},
				"RavenDBClientCertificate": &v1.RavenDBClientCertificate{},
			}[typeMeta.Kind]
			if !ok {
				t.Fatalf("sample has unknown kind %q", typeMeta.Kind)
			}
			if err := yaml.UnmarshalStrict(data, target); err != nil {
				t.Errorf("strict unmarshal failed: %v", err)
			}
		})
	}
}
