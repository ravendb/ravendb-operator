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

func baseClusterForExternalAccessTypesTest(name string) *RavenDBCluster {
	email := "user@example.com"
	certSecretRef := "ravendb-certs-a"
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
			Domain:               "example.com",
			ClusterCertSecretRef: &certSecretRef,
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
			ExternalAccessConfiguration: &ExternalAccessConfiguration{
				Type: "ingress-controller",
				IngressControllerExternalAccess: &IngressControllerContext{
					IngressClassName: "nginx",
					AdditionalAnnotations: map[string]string{
						"nginx.ingress.kubernetes.io/limit-connections": "10",
						"nginx.ingress.kubernetes.io/limit-rps":         "5",
					},
				},
			},
		},
	}
}

func TestExternalAccessTypeValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid type - aws",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "aws-nlb"
			},
			ExpectError: false,
		},
		{
			Name: "valid type - ingress-controller",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "invalid type",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "unknown"
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.externalAccessConfiguration.type"},
		},
		{
			Name: "missing type",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration = &ExternalAccessConfiguration{
					IngressControllerExternalAccess: &IngressControllerContext{
						IngressClassName: "nginx",
					},
				}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.externalAccessConfiguration.type"},
		},
	}

	runSpecValidationTest(t, baseClusterForExternalAccessTypesTest, testCases)
}

func TestAWSExternalAccessContext(t *testing.T) {

	testCases := []SpecValidationCase{
		{
			Name: "valid AWS nodeMappings",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "aws-nlb"
				spec.ExternalAccessConfiguration.AWSExternalAccess = &AWSExternalAccessContext{
					NodeMappings: []AWSNodeMapping{
						{
							Tag:             "A",
							EIPAllocationId: "eipalloc-0123456789abcdef0",
							SubnetId:        "subnet-abcdef1234567890",
						},
					},
				}
			},
			ExpectError: false,
		},
		{
			Name: "invalid EIP and Subnet format",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "aws-nlb"
				spec.ExternalAccessConfiguration.AWSExternalAccess = &AWSExternalAccessContext{
					NodeMappings: []AWSNodeMapping{
						{
							Tag:             "A",
							EIPAllocationId: "wrong-format",
							SubnetId:        "bad-subnet",
						},
					},
				}
			},
			ExpectError: true,
			ErrorParts: []string{
				"spec.externalAccessConfiguration.awsExternalAccessContext.nodeMappings[0].eipAllocationId",
				"spec.externalAccessConfiguration.awsExternalAccessContext.nodeMappings[0].subnetId",
			},
		},
		{
			Name: "missing tag field in mapping",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "aws-nlb"
				spec.ExternalAccessConfiguration.AWSExternalAccess = &AWSExternalAccessContext{
					NodeMappings: []AWSNodeMapping{
						{
							EIPAllocationId: "eipalloc-0123456789abcdef0",
							SubnetId:        "subnet-abcdef1234567890",
						},
					},
				}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.externalAccessConfiguration.awsExternalAccessContext.nodeMappings[0].tag"},
		},
	}

	runSpecValidationTest(t, baseClusterForExternalAccessTypesTest, testCases)
}

func TestAzureExternalAccessContext(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid Azure nodeMappings",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "azure-lb"
				spec.ExternalAccessConfiguration.AzureExternalAccess = &AzureExternalAccessContext{
					NodeMappings: []AzureNodeMapping{
						{
							Tag: "A",
							IP:  "192.168.1.10",
						},
					},
				}
			},
			ExpectError: false,
		},
		{
			Name: "missing IP field in mapping",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "azure-lb"
				spec.ExternalAccessConfiguration.AzureExternalAccess = &AzureExternalAccessContext{
					NodeMappings: []AzureNodeMapping{
						{
							Tag: "A",
						},
					},
				}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.externalAccessConfiguration.azureExternalAccessContext.nodeMappings[0].ip"},
		},
		{
			Name: "missing tag field in mapping",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "azure-lb"
				spec.ExternalAccessConfiguration.AzureExternalAccess = &AzureExternalAccessContext{
					NodeMappings: []AzureNodeMapping{
						{
							IP: "1.2.3.4",
						},
					},
				}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.externalAccessConfiguration.azureExternalAccessContext.nodeMappings[0].tag"},
		},
		{
			Name: "invalid IP format",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "azure-lb"
				spec.ExternalAccessConfiguration.AzureExternalAccess = &AzureExternalAccessContext{
					NodeMappings: []AzureNodeMapping{
						{
							Tag: "A",
							IP:  "999.999.999.999",
						},
					},
				}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.externalAccessConfiguration.azureExternalAccessContext.nodeMappings[0].ip"},
		},
	}

	runSpecValidationTest(t, baseClusterForExternalAccessTypesTest, testCases)
}

func TestIngressControllerContextValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid ingress controller context",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "missing ingress class name",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "ingress-controller"
				spec.ExternalAccessConfiguration.IngressControllerExternalAccess = &IngressControllerContext{}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.externalAccessConfiguration.ingressControllerContext.ingressClassName"},
		},
		{
			Name: "invalid ingress class name",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.ExternalAccessConfiguration.Type = "ingress-controller"
				spec.ExternalAccessConfiguration.IngressControllerExternalAccess = &IngressControllerContext{
					IngressClassName: "thegoldenplatypusIC",
				}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.externalAccessConfiguration.ingressControllerContext.ingressClassName"},
		},
	}

	runSpecValidationTest(t, baseClusterForExternalAccessTypesTest, testCases)
}
