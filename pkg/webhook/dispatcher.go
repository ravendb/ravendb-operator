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

package webhook

import (
	"context"
	"fmt"
	"ravendb-operator/pkg/webhook/mutator"
	"ravendb-operator/pkg/webhook/validator"
)

type ClusterAdapter = validator.ClusterAdapter

// Default is the defaulter webhook entrypoint.
// It calls the mutator pipeline, which currently has no registered mutators,
// so this function is effectively a no-op. We keep the call in place so that
// future defaults can be plugged in without changing the webhook wiring.
func Default(cluster ClusterAdapter) error {
	warnings, err := mutator.Run(cluster)
	for _, w := range warnings {
		fmt.Println(w)
	}
	return err
}

func ValidateCreate(ctx context.Context, cluster ClusterAdapter) error {
	return validator.RunCreate(ctx, cluster)
}

func ValidateUpdate(ctx context.Context, oldCluster, newCluster ClusterAdapter) error {
	return validator.RunUpdate(ctx, oldCluster, newCluster)
}
