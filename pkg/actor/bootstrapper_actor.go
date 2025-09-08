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
	"fmt"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"ravendb-operator/pkg/resource"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type BootstrapperActor struct {
	builder resource.PerClusterBuilder
}

func NewBootstrapperActor(builder resource.PerClusterBuilder) PerClusterActor {
	return &BootstrapperActor{builder: builder}
}

func (actor *BootstrapperActor) Name() string {
	return "BootstrapperActor"
}

func (actor *BootstrapperActor) Act(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster, client client.Client, scheme *runtime.Scheme) (bool, error) {
	bs, err := actor.builder.Build(ctx, cluster)
	if err != nil {
		return false, fmt.Errorf("failed to build bootstrapper resource: %w", err)
	}

	if err := controllerutil.SetControllerReference(cluster, bs, scheme); err != nil {
		return false, fmt.Errorf("set owner ref on bootstrapper resource: %w", err)
	}

	if _, ok := bs.(*batchv1.Job); ok {
		_, err := applyResourceSSA(ctx, client, bs, "ravendb-operator/job")
		return false, err
	}

	_, err = applyResourceSSA(ctx, client, bs, "ravendb-operator/cluster")
	return false, err
}

func (actor *BootstrapperActor) ShouldAct(cluster *ravendbv1alpha1.RavenDBCluster) bool {
	return !cluster.IsBootstrapped()
}
