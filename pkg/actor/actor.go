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

package actor

import (
	"context"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Actor interface {
	Name() string
}

type PerNodeActor interface {
	Actor
	Act(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode, c client.Client, scheme *runtime.Scheme) (bool, error)
}

type PerClusterActor interface {
	Actor
	ShouldAct(cluster *ravendbv1alpha1.RavenDBCluster) bool
	Act(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster, c client.Client, scheme *runtime.Scheme) (bool, error)
}
