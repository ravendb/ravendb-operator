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

package validator

import (
	"context"
	"fmt"
	"path/filepath"
	"ravendb-operator/pkg/webhook/adapter"
	"strings"

	storagev1alpha1 "k8s.io/api/storage/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type storageValidator struct {
	client client.Reader
}

func NewStorageValidator(c client.Reader) *storageValidator {
	return &storageValidator{client: c}
}

func (v *storageValidator) Name() string {
	return "storage-validator"
}

func (v *storageValidator) ValidateCreate(ctx context.Context, c adapter.ClusterAdapter) error {
	var errs []string

	dataSC := c.GetStorageDataStorageClassName()
	dataAM := c.GetStorageDataAccessModes()
	dataVAC := c.GetStorageDataVAC()

	logsRSC := c.GetLogsRavenStorageClassName()
	logsRAM := c.GetLogsRavenAccessModes()
	logsRVAC := c.GetLogsRavenVAC()
	logsRPath := c.GetLogsRavenPath()

	logsASC := c.GetLogsAuditStorageClassName()
	logsAAM := c.GetLogsAuditAccessModes()
	logsAVAC := c.GetLogsAuditVAC()
	logsAPath := c.GetLogsAuditPath()

	aVolNames := c.GetAdditionalVolumeNames()
	aVolMounts := c.GetAdditionalVolumeMountPaths()
	aVolSubPaths := c.GetAdditionalVolumeSubPaths()
	aVolSources := c.GetAdditionalVolumeSources()

	errs = append(errs, v.ValidateVolumeSpec(ctx, "spec.storage.data", dataSC, dataAM, dataVAC)...)

	errs = append(errs, v.ValidateVolumeSpec(ctx, "spec.storage.logs.ravendb", logsRSC, logsRAM, logsRVAC)...)
	errs = append(errs, ValidateAbsolutePath("spec.storage.logs.ravendb.path", logsRPath)...)

	errs = append(errs, v.ValidateVolumeSpec(ctx, "spec.storage.logs.audit", logsASC, logsAAM, logsAVAC)...)
	errs = append(errs, ValidateAbsolutePath("spec.storage.logs.audit.path", logsAPath)...)

	errs = append(errs, ValidateAdditionalVolumes("spec.storage.additionalVolumes", aVolNames, aVolMounts, aVolSubPaths, aVolSources)...)

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func (v *storageValidator) ValidateUpdate(ctx context.Context, _, newC ClusterAdapter) error {
	return v.ValidateCreate(ctx, newC)
}

func ValidateAbsolutePath(fieldPath string, pathPtr *string) []string {
	if pathPtr == nil {
		return nil
	}
	if !filepath.IsAbs(*pathPtr) {
		return []string{fmt.Sprintf("%s must be an absolute path", fieldPath)}
	}
	return nil
}

func (v *storageValidator) ValidateVolumeSpec(ctx context.Context, path string, storageClass *string, accessModes []string, vac *string) []string {
	var errs []string

	// todo: implement storage class validation in dedicated issue
	// if storageClass == nil {
	// 	errs = append(errs, fmt.Sprintf("warning: %s.storageClassName is not set - make sure a default StorageClass exists in the cluster or PVCs may remain Pending", path))
	// } else {
	// 	errs = append(errs, ValidateRWX(*storageClass, accessModes)...)
	// }

	if vac != nil {
		if err := v.ValidateVAC(ctx, *vac); err != nil {
			errs = append(errs, fmt.Sprintf("%s.volumeAttributesClassName '%s' does not reference a valid VolumeAttributesClass: %v", path, *vac, err))
		}
	}
	return errs
}

func ValidateRWX(sc string, accessModes []string) []string {
	var errs []string
	// TODO: implement rwx restriction based on known provisioner map
	return errs
}

func (v *storageValidator) ValidateVAC(ctx context.Context, name string) error {
	var vac storagev1alpha1.VolumeAttributesClass
	if err := v.client.Get(ctx, client.ObjectKey{Name: name}, &vac); err != nil {
		return fmt.Errorf("referenced VAC '%s' not found", name)
	}
	return nil
}

func ValidateAdditionalVolumes(path string, names []string, mounts []*string, subpaths []*string, sources []map[string]bool) []string {
	var errs []string
	seenNames := make(map[string]bool)

	for i, name := range names {
		p := fmt.Sprintf("%s[%d]", path, i)

		if seenNames[name] {
			errs = append(errs, fmt.Sprintf("%s.name must be unique — '%s' is used more than once", p, name))
		}
		seenNames[name] = true

		errs = append(errs, ValidateAbsolutePath(p+".mountPath", mounts[i])...)

		if subpaths[i] != nil && strings.ContainsAny(*subpaths[i], "/\\") {
			errs = append(errs, fmt.Sprintf("%s.subPath must be a file name only (no path separators)", p))
		}

		if len(sources[i]) != 1 {
			errs = append(errs, fmt.Sprintf("%s.volumeSource must have exactly one source (configMap, secret, or persistentVolumeClaim)", p))
		} else {
			for key := range sources[i] {
				if key != "configMap" && key != "secret" && key != "persistentVolumeClaim" {
					errs = append(errs, fmt.Sprintf("%s.volumeSource contains invalid type: '%s' — must be one of: configMap, secret, persistentVolumeClaim", p, key))
				}
			}
		}
	}

	return errs
}
