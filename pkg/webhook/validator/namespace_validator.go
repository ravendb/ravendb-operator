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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type singleClusterPerNamespaceValidator struct {
	client ctrlclient.Client
}

func NewSingleClusterPerNamespaceValidator(c ctrlclient.Client) Validator {
	return &singleClusterPerNamespaceValidator{client: c}
}

func (v *singleClusterPerNamespaceValidator) Name() string {
	return "single-cluster-per-namespace-validator"
}

func (v *singleClusterPerNamespaceValidator) ValidateCreate(ctx context.Context, cluster ClusterAdapter) error {
	return v.validateNamespaceUniqueness(ctx, cluster)
}

func (v *singleClusterPerNamespaceValidator) ValidateUpdate(ctx context.Context, oldCluster, newCluster ClusterAdapter) error {
	return v.validateNamespaceUniqueness(ctx, newCluster)
}

func (v *singleClusterPerNamespaceValidator) validateNamespaceUniqueness(ctx context.Context, cluster ClusterAdapter) error {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "ravendb.ravendb.io",
		Version: "v1",
		Kind:    "RavenDBClusterList",
	})

	if err := v.client.List(ctx, list, ctrlclient.InNamespace(cluster.GetNamespace())); err != nil {
		return fmt.Errorf("failed to list RavenDBClusters in namespace %q: %w", cluster.GetNamespace(), err)
	}

	for _, item := range list.Items {
		if item.GetName() == cluster.GetName() {
			continue
		}

		return fmt.Errorf(
			"only one RavenDBCluster is allowed per namespace: found existing RavenDBCluster %q in namespace %q",
			item.GetName(),
			cluster.GetNamespace(),
		)
	}

	return nil
}
