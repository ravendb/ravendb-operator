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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func baseClusterForStorageTypesTest(name string) *RavenDBCluster {
	email := "user@example.com"
	certSecretRef := "ravendb-certs-a"
	storageClass := "local-path"
	accessModes := []string{"ReadWriteOnce"}
	logPath := "/var/log/ravendb"
	volumeAttrClass := "raven-default"

	subPath := "logfile.txt"
	volumeName := "my-vol"
	mountPath := "/mnt/myvol"

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
					CertSecretRef:      &certSecretRef,
				},
			},
			StorageSpec: StorageSpec{
				Data: VolumeSpec{
					Size:                      "5Gi",
					StorageClassName:          &storageClass,
					AccessModes:               &accessModes,
					VolumeAttributesClassName: &volumeAttrClass,
				},
				Logs: &LogsSpec{
					RavenDB: &LogSettings{
						VolumeSpec: VolumeSpec{
							Size: "1Gi",
						},
						Path: &logPath,
					},
					Audit: &LogSettings{
						VolumeSpec: VolumeSpec{
							Size: "500Mi",
						},
					},
				},
				AdditionalVolumes: &[]AdditionalVolume{
					{
						Name:      volumeName,
						MountPath: mountPath,
						SubPath:   &subPath,
						VolumeSource: VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "my-config",
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestStorageDataFieldValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid data volume",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "missing data field",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Data = VolumeSpec{}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.storage.data.size"},
		},
	}

	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestStorageLogsFieldValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "logs field omitted",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Logs = nil
			},
			ExpectError: false,
		},
	}

	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestStorageAdditionalVolumesFieldValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "additionalVolumes omitted",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.AdditionalVolumes = nil
			},
			ExpectError: false,
		},
	}

	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestVolumeSpecSizeValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid size - Gi",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "valid size - Mi",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Data.Size = "512Mi"
			},
			ExpectError: false,
		},
		{
			Name: "invalid size - no unit",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Data.Size = "100"
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.storage.data.size"},
		},
		{
			Name: "invalid size - bad unit",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Data.Size = "1GB"
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.storage.data.size"},
		},
		{
			Name: "missing size",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Data.Size = ""
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.storage.data.size"},
		},
	}

	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestVolumeSpecStorageClassNameValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid storage class name",
			Modify: func(spec *RavenDBClusterSpec) {

			},
			ExpectError: false,
		},
		{
			Name: "unset storage class name",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Data.StorageClassName = nil
			},
			ExpectError: false,
		},
	}

	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestVolumeSpecAccessModesValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid single access mode - ReadWriteOnce",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "valid single access mode - ReadWriteMany",
			Modify: func(spec *RavenDBClusterSpec) {
				modes := []string{"ReadWriteMany"}
				spec.StorageSpec.Data.AccessModes = &modes
			},
			ExpectError: false,
		},
		{
			Name: "valid multiple access modes",
			Modify: func(spec *RavenDBClusterSpec) {
				modes := []string{"ReadWriteOnce", "ReadWriteMany"}
				spec.StorageSpec.Data.AccessModes = &modes
			},
			ExpectError: false,
		},
	}

	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestVolumeSpecVolumeAttributesClassNameValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "valid volumeAttributesClassName",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
		{
			Name: "empty volumeAttributesClassName",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Data.VolumeAttributesClassName = nil
			},
			ExpectError: false,
		},
	}

	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestLogsSpecValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "logs unset",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Logs = nil
			},
			ExpectError: false,
		},
		{
			Name: "logs with only ravenDB",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Logs = &LogsSpec{
					RavenDB: &LogSettings{
						VolumeSpec: VolumeSpec{
							Size: "1Gi",
						},
					},
				}
			},
			ExpectError: false,
		},
		{
			Name: "logs with only audit",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Logs = &LogsSpec{
					Audit: &LogSettings{
						VolumeSpec: VolumeSpec{
							Size: "1Gi",
						},
					},
				}
			},
			ExpectError: false,
		},
		{
			Name: "logs with both ravenDB and audit",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
	}

	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestLogSettingsPathValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "path not set",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.Logs = &LogsSpec{
					RavenDB: &LogSettings{
						VolumeSpec: VolumeSpec{
							Size: "1Gi",
						},
						Path: nil,
					},
				}
			},
			ExpectError: false,
		},
		{
			Name: "valid path set",
			Modify: func(spec *RavenDBClusterSpec) {
				p := "/var/log/ravendb"
				spec.StorageSpec.Logs = &LogsSpec{
					RavenDB: &LogSettings{
						VolumeSpec: VolumeSpec{
							Size: "1Gi",
						},
						Path: &p,
					},
				}
			},
			ExpectError: false,
		},
	}

	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestAdditionalVolumeNameValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "missing name",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.AdditionalVolumes = &[]AdditionalVolume{
					{
						Name:      "",
						MountPath: "/mnt/data",
						VolumeSource: VolumeSource{
							Secret: &corev1.SecretVolumeSource{SecretName: "my-secret"},
						},
					},
				}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.storage.additionalVolumes[0].name"},
		},
		{
			Name: "valid name",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
	}
	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestAdditionalVolumeMountPathValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "missing mountPath",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.AdditionalVolumes = &[]AdditionalVolume{
					{
						Name:      "extra",
						MountPath: "",
						VolumeSource: VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "cfg",
								},
							},
						},
					},
				}
			},
			ExpectError: true,
			ErrorParts:  []string{"spec.storage.additionalVolumes[0].mountPath"},
		},
		{
			Name: "valid mountPath",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.AdditionalVolumes = &[]AdditionalVolume{
					{
						Name:      "extra",
						MountPath: "/mnt/config",
						VolumeSource: VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "cfg",
								},
							},
						},
					},
				}
			},
			ExpectError: false,
		},
	}
	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}

func TestAdditionalVolumeSubPathValidation(t *testing.T) {
	testCases := []SpecValidationCase{
		{
			Name: "no subPath",
			Modify: func(spec *RavenDBClusterSpec) {
				spec.StorageSpec.AdditionalVolumes = &[]AdditionalVolume{
					{
						Name:      "extra",
						MountPath: "/mnt/data",
						SubPath:   nil,
						VolumeSource: VolumeSource{
							Secret: &corev1.SecretVolumeSource{SecretName: "s"},
						},
					},
				}
			},
			ExpectError: false,
		},
		{
			Name: "valid subPath",
			Modify: func(spec *RavenDBClusterSpec) {
			},
			ExpectError: false,
		},
	}
	runSpecValidationTest(t, baseClusterForStorageTypesTest, testCases)
}
