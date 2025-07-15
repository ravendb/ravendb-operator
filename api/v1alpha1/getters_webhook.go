package v1alpha1

func (r *RavenDBCluster) GetImage() string {
	return r.Spec.Image
}

func (r *RavenDBCluster) GetIpp() string {
	return r.Spec.ImagePullPolicy
}

func (r *RavenDBCluster) SetIpp(val string) {
	r.Spec.ImagePullPolicy = val
}

func (r *RavenDBCluster) GetMode() string {
	return string(r.Spec.Mode)
}

func (r *RavenDBCluster) GetEmail() string {
	if r.Spec.Email == nil {
		return ""
	}
	return *r.Spec.Email
}
func (r *RavenDBCluster) GetDomain() string {
	return r.Spec.Domain
}

func (r *RavenDBCluster) GetClusterCertsSecretRef() string {
	if r.Spec.ClusterCertSecretRef == nil {
		return ""
	}
	return *r.Spec.ClusterCertSecretRef
}

func (r *RavenDBCluster) GetEnv() map[string]string {
	return r.Spec.Env
}

func (r *RavenDBCluster) GetLicenseSecretRef() string {
	return r.Spec.LicenseSecretRef
}

func mapNodes[T any](r *RavenDBCluster, f func(n RavenDBNode) T) []T {
	out := make([]T, len(r.Spec.Nodes))
	for i, n := range r.Spec.Nodes {
		out[i] = f(n)
	}
	return out
}

func (r *RavenDBCluster) GetNodeTags() []string {
	return mapNodes(r, func(n RavenDBNode) string { return n.Tag })
}

func (r *RavenDBCluster) GetNodePublicUrls() []string {
	return mapNodes(r, func(n RavenDBNode) string { return n.PublicServerUrl })
}

func (r *RavenDBCluster) GetNodeTcpUrls() []string {
	return mapNodes(r, func(n RavenDBNode) string { return n.PublicServerUrlTcp })
}

func (r *RavenDBCluster) GetNodeCertSecretRefs() []*string {
	return mapNodes(r, func(n RavenDBNode) *string {
		return n.CertSecretRef
	})
}

func (r *RavenDBCluster) IsExternalAccessSet() bool {
	return r.Spec.ExternalAccessConfiguration != nil
}

func (r *RavenDBCluster) GetExternalAccessType() string {
	if r.Spec.ExternalAccessConfiguration == nil {
		return ""
	}
	return string(r.Spec.ExternalAccessConfiguration.Type)
}

func (r *RavenDBCluster) GetIngressClassName() string {
	if r.Spec.ExternalAccessConfiguration == nil {
		return ""
	}
	ctx := r.Spec.ExternalAccessConfiguration.IngressControllerExternalAccess
	if ctx == nil {
		return ""
	}
	return ctx.IngressClassName
}

func (r *RavenDBCluster) GetIngressAnnotations() map[string]string {
	if r.Spec.ExternalAccessConfiguration == nil {
		return nil
	}
	ctx := r.Spec.ExternalAccessConfiguration.IngressControllerExternalAccess
	if ctx == nil {
		return nil
	}
	return ctx.AdditionalAnnotations
}

func (r *RavenDBCluster) IsIngressContextSet() bool {
	return r.Spec.ExternalAccessConfiguration != nil &&
		r.Spec.ExternalAccessConfiguration.IngressControllerExternalAccess != nil
}

func (r *RavenDBCluster) IsAWSContextSet() bool {
	return r.Spec.ExternalAccessConfiguration != nil &&
		r.Spec.ExternalAccessConfiguration.AWSExternalAccess != nil
}

func (r *RavenDBCluster) GetStorageDataStorageClassName() *string {
	return r.Spec.StorageSpec.Data.StorageClassName
}

func (r *RavenDBCluster) GetStorageDataAccessModes() []string {
	if r.Spec.StorageSpec.Data.AccessModes == nil {
		return []string{}
	}
	return *r.Spec.StorageSpec.Data.AccessModes
}

func (r *RavenDBCluster) GetStorageDataVAC() *string {
	return r.Spec.StorageSpec.Data.VolumeAttributesClassName
}

func (r *RavenDBCluster) GetLogsRavenStorageClassName() *string {
	if r.Spec.StorageSpec.Logs == nil || r.Spec.StorageSpec.Logs.RavenDB == nil {
		return nil
	}
	return r.Spec.StorageSpec.Logs.RavenDB.StorageClassName
}

func (r *RavenDBCluster) GetLogsRavenAccessModes() []string {
	if r.Spec.StorageSpec.Logs == nil ||
		r.Spec.StorageSpec.Logs.RavenDB == nil ||
		r.Spec.StorageSpec.Logs.RavenDB.AccessModes == nil {
		return []string{}
	}
	return *r.Spec.StorageSpec.Logs.RavenDB.AccessModes
}

func (r *RavenDBCluster) GetLogsRavenVAC() *string {
	if r.Spec.StorageSpec.Logs == nil || r.Spec.StorageSpec.Logs.RavenDB == nil {
		return nil
	}
	return r.Spec.StorageSpec.Logs.RavenDB.VolumeAttributesClassName
}

func (r *RavenDBCluster) GetLogsRavenPath() *string {
	if r.Spec.StorageSpec.Logs == nil || r.Spec.StorageSpec.Logs.RavenDB == nil {
		return nil
	}
	return r.Spec.StorageSpec.Logs.RavenDB.Path
}

func (r *RavenDBCluster) GetLogsAuditStorageClassName() *string {
	if r.Spec.StorageSpec.Logs == nil || r.Spec.StorageSpec.Logs.Audit == nil {
		return nil
	}
	return r.Spec.StorageSpec.Logs.Audit.StorageClassName
}

func (r *RavenDBCluster) GetLogsAuditAccessModes() []string {
	if r.Spec.StorageSpec.Logs == nil ||
		r.Spec.StorageSpec.Logs.Audit == nil ||
		r.Spec.StorageSpec.Logs.Audit.AccessModes == nil {
		return []string{}
	}
	return *r.Spec.StorageSpec.Logs.Audit.AccessModes
}

func (r *RavenDBCluster) GetLogsAuditVAC() *string {
	if r.Spec.StorageSpec.Logs == nil || r.Spec.StorageSpec.Logs.Audit == nil {
		return nil
	}
	return r.Spec.StorageSpec.Logs.Audit.VolumeAttributesClassName
}

func (r *RavenDBCluster) GetLogsAuditPath() *string {
	if r.Spec.StorageSpec.Logs == nil || r.Spec.StorageSpec.Logs.Audit == nil {
		return nil
	}
	return r.Spec.StorageSpec.Logs.Audit.Path
}

func (r *RavenDBCluster) GetAdditionalVolumeNames() []string {
	if r.Spec.StorageSpec.AdditionalVolumes == nil {
		return []string{}
	}
	var out []string
	for _, v := range *r.Spec.StorageSpec.AdditionalVolumes {
		out = append(out, v.Name)
	}
	return out
}

func (r *RavenDBCluster) GetAdditionalVolumeMountPaths() []*string {
	if r.Spec.StorageSpec.AdditionalVolumes == nil {
		return []*string{}
	}
	var out []*string
	for i := range *r.Spec.StorageSpec.AdditionalVolumes {
		out = append(out, &(*r.Spec.StorageSpec.AdditionalVolumes)[i].MountPath)
	}
	return out
}

func (r *RavenDBCluster) GetAdditionalVolumeSubPaths() []*string {
	if r.Spec.StorageSpec.AdditionalVolumes == nil {
		return []*string{}
	}
	var out []*string
	for _, v := range *r.Spec.StorageSpec.AdditionalVolumes {
		out = append(out, v.SubPath)
	}
	return out
}

func (r *RavenDBCluster) GetAdditionalVolumeSources() []map[string]bool {
	if r.Spec.StorageSpec.AdditionalVolumes == nil {
		return []map[string]bool{}
	}
	var result []map[string]bool
	for _, v := range *r.Spec.StorageSpec.AdditionalVolumes {
		entry := make(map[string]bool)
		if v.VolumeSource.ConfigMap != nil {
			entry["configMap"] = true
		}
		if v.VolumeSource.Secret != nil {
			entry["secret"] = true
		}
		if v.VolumeSource.PersistentVolumeClaim != nil {
			entry["persistentVolumeClaim"] = true
		}
		result = append(result, entry)
	}
	return result
}
