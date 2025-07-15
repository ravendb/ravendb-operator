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
	"ravendb-operator/pkg/webhook/adapter"

	"k8s.io/apimachinery/pkg/util/errors"
)

type ClusterAdapter = adapter.ClusterAdapter

type Validator interface {
	Name() string
	ValidateCreate(ctx context.Context, cluster ClusterAdapter) error
	ValidateUpdate(ctx context.Context, oldCluster, newCluster ClusterAdapter) error
}

var validators []Validator

func Register(v Validator) {
	validators = append(validators, v)
}

func RunCreate(ctx context.Context, cluster ClusterAdapter) error {
	var errs []error
	for _, v := range validators {
		if err := v.ValidateCreate(ctx, cluster); err != nil {
			errs = append(errs, fmt.Errorf("\n[%s] %w", v.Name(), err))
		}
	}
	return errors.NewAggregate(errs)
}

func RunUpdate(ctx context.Context, oldCluster, newCluster ClusterAdapter) error {
	var errs []error
	for _, v := range validators {
		if err := v.ValidateUpdate(ctx, oldCluster, newCluster); err != nil {
			errs = append(errs, fmt.Errorf("\n[%s] %w", v.Name(), err))
		}
	}
	return errors.NewAggregate(errs)
}
