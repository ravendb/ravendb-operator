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
	"net"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type generalValidator struct {
	client client.Reader
}

func NewGeneralValidator(c client.Reader) *generalValidator {
	return &generalValidator{client: c}
}

func (v *generalValidator) Name() string {
	return "general-validator"
}

func (v *generalValidator) ValidateCreate(ctx context.Context, c ClusterAdapter) error {
	var errs []string

	mode := c.GetMode()
	email := c.GetEmail()
	domain := c.GetDomain()
	license := c.GetLicenseSecretRef()
	clusterCert := c.GetClusterCertsSecretRef()
	envVars := c.GetEnv()

	errs = append(errs, ValidateEmail(mode, email)...)
	errs = append(errs, ValidateLicenseSecret(v, ctx, license)...)
	errs = append(errs, ValidateClusterCertSecret(v, ctx, mode, clusterCert)...)
	errs = append(errs, ValidateDomain(domain)...)
	errs = append(errs, ValidateEnv(envVars)...)

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}

	return nil

}

func (v *generalValidator) ValidateUpdate(ctx context.Context, _, newC ClusterAdapter) error {
	return v.ValidateCreate(ctx, newC)
}

func ValidateEmail(mode, email string) []string {
	var errs []string

	if mode == "LetsEncrypt" && email == "" {
		errs = append(errs, "spec.email is required when mode is LetsEncrypt")
	}
	if mode == "None" && email != "" {
		errs = append(errs, "spec.email must not be set when mode is None")
	}

	return errs
}

func ValidateLicenseSecret(v *generalValidator, ctx context.Context, license string) []string {
	var errs []string

	secret, err := v.getSecret(ctx, license)
	if err != nil {
		errs = append(errs, fmt.Sprintf("spec.licenseSecretRef: %v", err))
		return errs
	}

	if len(secret.Data) != 1 {
		errs = append(errs, fmt.Sprintf("spec.licenseSecretRef: secret '%s' must contain exactly one '.json' file", license))
		return errs
	}

	for key := range secret.Data {
		if !strings.HasSuffix(key, ".json") {
			errs = append(errs, fmt.Sprintf("spec.licenseSecretRef: secret '%s' must contain a file ending with '.json', got '%s' instead", license, key))
		}
		break
	}
	return errs
}

func ValidateClusterCertSecret(v *generalValidator, ctx context.Context, mode, clusterCert string) []string {
	var errs []string

	if mode == "LetsEncrypt" {
		if clusterCert != "" {
			errs = append(errs, "spec.clusterCertSecretRef must not be set when mode is LetsEncrypt")
		}
		return errs
	}

	if mode == "None" && clusterCert == "" {
		errs = append(errs, "spec.clusterCertSecretRef is required when mode is None")
		return errs
	}

	secret, err := v.getSecret(ctx, clusterCert)
	if err != nil {
		errs = append(errs, fmt.Sprintf("spec.clusterCertSecretRef: %v", err))
		return errs
	}

	if len(secret.Data) != 1 {
		errs = append(errs, fmt.Sprintf("spec.clusterCertSecretRef: secret '%s' must contain exactly one '.pfx' file", clusterCert))
		return errs
	}

	for key := range secret.Data {
		if !strings.HasSuffix(key, ".pfx") {
			errs = append(errs, fmt.Sprintf("spec.clusterCertSecretRef: secret '%s' must contain a file ending with '.pfx , got '%s' instead", clusterCert, key))
		}
		break
	}

	return errs
}

func ValidateDomain(domain string) []string {
	var errs []string

	if !isValidFQDN(domain) {
		errs = append(errs, fmt.Sprintf("spec.domain '%s' must be a valid FQDN", domain))
	}

	return errs
}

func ValidateEnv(envVars map[string]string) []string {
	var errs []string
	duplicateEvars := map[string]bool{}

	for name := range envVars {
		if duplicateEvars[name] {
			errs = append(errs, fmt.Sprintf("spec.env: duplicate environment variable '%s'", name))
		}
		duplicateEvars[name] = true

		if !strings.HasPrefix(name, "RAVEN_") {
			errs = append(errs, fmt.Sprintf("spec.env: environment variable '%s' must start with 'RAVEN_'", name))
		}
	}

	return errs
}

func isValidFQDN(s string) bool {
	if strings.Contains(s, "_") || s == "localhost" {
		return false
	}
	ip := net.ParseIP(s)
	return ip == nil
}

func (v *generalValidator) getSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	var secret corev1.Secret
	if err := v.client.Get(ctx, client.ObjectKey{Name: name, Namespace: "ravendb"}, &secret); err != nil {
		return nil, fmt.Errorf("secret '%s' not found", name)
	}
	return &secret, nil
}
