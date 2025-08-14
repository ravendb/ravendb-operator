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

package director

import (
	"context"
	"fmt"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"ravendb-operator/pkg/actor"
	"ravendb-operator/pkg/resource"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Director interface {
	ExecutePerCluster(
		ctx context.Context,
		cluster *ravendbv1alpha1.RavenDBCluster,
		c client.Client,
		scheme *runtime.Scheme,
	) error

	ExecutePerNode(
		ctx context.Context,
		cluster *ravendbv1alpha1.RavenDBCluster,
		node ravendbv1alpha1.RavenDBNode,
		c client.Client,
		scheme *runtime.Scheme,
	) error
}

type DefaultDirector struct {
	perNodeActors    []actor.PerNodeActor
	perClusterActors []actor.PerClusterActor
}

func NewDefaultDirector() Director {
	return &DefaultDirector{
		perClusterActors: []actor.PerClusterActor{
			actor.NewIngressActor(resource.NewIngressBuilder()),
			actor.NewBootstrapperActor(resource.NewJobBuilder()),
		},
		perNodeActors: []actor.PerNodeActor{
			actor.NewStatefulSetActor(resource.NewStatefulSetBuilder()),
			actor.NewServiceActor(resource.NewServiceBuilder()),
		},
	}
}

func (d *DefaultDirector) ExecutePerCluster(
	ctx context.Context,
	cluster *ravendbv1alpha1.RavenDBCluster,
	c client.Client,
	scheme *runtime.Scheme,
) error {
	for _, actor := range d.perClusterActors {
		if !actor.ShouldAct(cluster) {
			continue
		}
		if err := actor.Act(ctx, cluster, c, scheme); err != nil {
			return fmt.Errorf("%s failed: %w", actor.Name(), err)
		}
	}
	return nil
}

func (d *DefaultDirector) ExecutePerNode(
	ctx context.Context,
	cluster *ravendbv1alpha1.RavenDBCluster,
	node ravendbv1alpha1.RavenDBNode,
	c client.Client,
	scheme *runtime.Scheme,
) error {
	for _, actor := range d.perNodeActors {
		if err := actor.Act(ctx, cluster, node, c, scheme); err != nil {
			return fmt.Errorf("%s failed: %w", actor.Name(), err)
		}
	}
	return nil
}
