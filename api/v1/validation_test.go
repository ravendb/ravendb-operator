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

package v1

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func runSpecValidationTest(t *testing.T, base func(name string) *RavenDBCluster, testCases []SpecValidationCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			sanitizedName := sanitizeName("test-" + tc.Name)
			instance := base(sanitizedName)
			tc.Modify(&instance.Spec)

			err := k8sClient.Create(ctx, instance)
			if tc.ExpectError {
				if err == nil && len(tc.ErrorParts) > 0 {
					t.Skip("skipping...") // MinItems=1, MinLength=1 is enforced at the api server level, envtest does not
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
