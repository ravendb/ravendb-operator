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

package v1alpha1

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func baseClusterForNodeTypesTest(name string) *RavenDBCluster {
	email := "user@example.com"
	certSecretRef := "ravendb-certs-a"
	return &RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: RavenDBClusterSpec{
			Image:               "ravendb/ravendb:latest",
			ImagePullPolicy:     "Always",
			Mode:                "None",
			Email:               &email,
			LicenseSecretRef:    "license-secret",
			Domain:              "example.com",
			ClientCertSecretRef: "client-cert",
			Nodes: []RavenDBNode{
				{
					Tag:                "A",
					PublicServerUrl:    "https://a.example.com",
					PublicServerUrlTcp: "tcp://a-tcp.example.com",
					CertSecretRef:      &certSecretRef,
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

func TestNodeTagValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid tag",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "missing tag",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Nodes[0].Tag = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.nodes[0].tag"},
		},
		{
			Name: "tag too long",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Nodes[0].Tag = "ABCDE"
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.nodes[0].tag"},
		},
	}

	runSpecValidationTest(t, baseClusterForNodeTypesTest, testCases)
}

func TestPublicServerUrlValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid publicServerUrl",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "missing publicServerUrl",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Nodes[0].PublicServerUrl = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.nodes[0].publicServerUrl"},
		},
	}

	runSpecValidationTest(t, baseClusterForNodeTypesTest, testCases)
}

func TestPublicServerUrlTcpValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid publicServerUrlTcp",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "missing publicServerUrlTcp",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Nodes[0].PublicServerUrlTcp = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.nodes[0].publicServerUrlTcp"},
		},
	}

	runSpecValidationTest(t, baseClusterForNodeTypesTest, testCases)
}

func TestCertSecretRefValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid certSecretRef",
			Modify: func(spec *RavenDBClusterSpec) {

			},
			ExpectError: false,
		},
		{
			Name: "nil certSecretRef (omitted)",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.Nodes[0].CertSecretRef = nil
			},
			ExpectError: false,
		},
	}

	runSpecValidationTest(t, baseClusterForNodeTypesTest, testCases)
}
