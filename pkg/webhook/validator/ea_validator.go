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
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type eaValidator struct {
	client client.Reader
}

func NewEaValidator(c client.Reader) *eaValidator {
	return &eaValidator{client: c}
}

func (v *eaValidator) Name() string {
	return "externalAccess-validator"
}

func (v *eaValidator) ValidateCreate(ctx context.Context, c ClusterAdapter) error {
	var errs []string

	if !c.IsExternalAccessSet() {
		return nil
	}

	typeVal := c.GetExternalAccessType()
	ingressSet := c.IsIngressContextSet()
	awsSet := c.IsAWSContextSet()
	azureSet := c.IsAzureContextSet()
	annotations := c.GetIngressAnnotations()

	switch typeVal {
	case "aws-nlb":
		if !awsSet {
			errs = append(errs, "spec.externalAccessConfiguration.awsExternalAccessContext is required when type is 'aws-nlb'")
		}
		if ingressSet {
			errs = append(errs, "spec.externalAccessConfiguration.ingressControllerContext must not be set when type is 'aws-nlb'")
		}
		if azureSet {
			errs = append(errs, "spec.externalAccessConfiguration.azureExternalAccessContext must not be set when type is 'aws-nlb'")
		}

	case "azure-lb":
		if !azureSet {
			errs = append(errs, "spec.externalAccessConfiguration.azureExternalAccessContext is required when type is 'azure-lb'")
		}
		if ingressSet {
			errs = append(errs, "spec.externalAccessConfiguration.ingressControllerContext must not be set when type is 'azure-lb'")
		}
		if awsSet {
			errs = append(errs, "spec.externalAccessConfiguration.awsExternalAccessContext must not be set when type is 'azure-lb'")
		}

	case "ingress-controller":
		if !ingressSet {
			errs = append(errs, "spec.externalAccessConfiguration.ingressControllerContext is required when type is 'ingress-controller'")
		}
		if awsSet {
			errs = append(errs, "spec.externalAccessConfiguration.awsExternalAccessContext must not be set when type is 'ingress-controller'")
		}
		if azureSet {
			errs = append(errs, "spec.externalAccessConfiguration.azureExternalAccessContext must not be set when type is 'ingress-controller'")
		}

		errs = append(errs, validateIngressAnnotations(annotations)...)

	default:
		errs = append(errs, fmt.Sprintf("spec.externalAccessConfiguration.type has invalid value: '%s'", typeVal))
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func (v *eaValidator) ValidateUpdate(ctx context.Context, _, newC ClusterAdapter) error {
	return v.ValidateCreate(ctx, newC)
}

func validateIngressAnnotations(annotations map[string]string) []string {
	var errs []string

	nginxKey := "nginx.ingress.kubernetes.io/ssl-passthrough"
	haproxyKey := "haproxy.org/ssl-passthrough"
	for k, v := range annotations {
		switch k {
		case nginxKey, haproxyKey:
			if v == "false" {
				errs = append(errs, fmt.Sprintf(
					"spec.externalAccessConfiguration.ingressControllerContext.additionalAnnotations must not contain '%s: \"false\"'",
					k))
			}
		}
	}
	return errs
}
