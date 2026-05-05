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
	pattern := filepath.Join(repoRoot, "config/samples/ravendb_v1_ravendbcluster*.yaml")

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
			var c v1.RavenDBCluster
			if err := yaml.UnmarshalStrict(data, &c); err != nil {
				t.Errorf("strict unmarshal failed: %v", err)
			}
		})
	}
}
