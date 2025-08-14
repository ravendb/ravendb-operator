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

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type BootstrapperActor struct {
	builder resource.PerClusterBuilder
}

func NewBootstrapperActor(builder resource.PerClusterBuilder) PerClusterActor {
	return &BootstrapperActor{builder: builder}
}

func (a *BootstrapperActor) Name() string {
	return "BootstrapperActor"
}

func (a *BootstrapperActor) Act(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster, c client.Client, scheme *runtime.Scheme) error {
	bs, err := a.builder.Build(ctx, cluster)
	if err != nil {
		return fmt.Errorf("failed to build Bootstrapper job: %w", err)
	}

	if job, ok := bs.(*batchv1.Job); ok {
		return ensureJobCreateOnly(ctx, c, scheme, job, cluster)
	}

	if err := applyResource(ctx, c, scheme, bs); err != nil {
		return fmt.Errorf("failed to apply Bootstrapper resource: %w", err)
	}
	return nil
}

func (a *BootstrapperActor) ShouldAct(cluster *ravendbv1alpha1.RavenDBCluster) bool {
	return !cluster.IsBootstrapped()
}

func ensureJobCreateOnly(ctx context.Context, c client.Client, scheme *runtime.Scheme, job *batchv1.Job, owner client.Object) error {
	var existing batchv1.Job
	err := c.Get(ctx, types.NamespacedName{Namespace: job.Namespace, Name: job.Name}, &existing)

	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("get job: %w", err)
	}
	if err := controllerutil.SetControllerReference(owner, job, scheme); err != nil {
		return fmt.Errorf("set ownerRef: %w", err)
	}
	if err := c.Create(ctx, job); err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}
