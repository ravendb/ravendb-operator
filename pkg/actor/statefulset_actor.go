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

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/resource"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type StatefulSetActor struct {
	builder resource.PerNodeBuilder
}

func NewStatefulSetActor(builder resource.PerNodeBuilder) PerNodeActor {
	return &StatefulSetActor{builder: builder}
}

func (actor *StatefulSetActor) Name() string {
	return "StatefulSetActor"
}

// Act applies the node selected by the Upgrader using SSA. Node selection is
// the rollout barrier: the controller never calls this actor for unselected
// peers, so the actor must apply the selected node's desired image directly.
//
// Do not make image application depend on reading the rollout marker here. The
// Upgrader persists that marker immediately before this call, while the
// controller-runtime client reads through an eventually consistent cache. A
// read-after-write can therefore observe the pre-marker StatefulSet, freeze
// the old image, and let post-gates pass without performing the rollout.
//
// Existing metadata annotations are preserved so durable coordination markers
// and user annotations survive the server-side apply.
func (actor *StatefulSetActor) Act(ctx context.Context, cluster *ravendbv1.RavenDBCluster, node ravendbv1.RavenDBNode, kc client.Client, scheme *runtime.Scheme) (bool, error) {
	sts, err := actor.builder.Build(ctx, cluster, node)
	if err != nil {
		return false, fmt.Errorf("failed to build StatefulSet: %w", err)
	}

	desired, ok := sts.(*appsv1.StatefulSet)
	if !ok {
		return false, fmt.Errorf("builder returned %T, expected *appsv1.StatefulSet", sts)
	}

	var existing appsv1.StatefulSet
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: desired.GetName()}
	haveExisting := (kc.Get(ctx, key, &existing) == nil)

	if haveExisting {
		if desired.Annotations == nil {
			desired.Annotations = map[string]string{}
		}
		for k, v := range existing.Annotations {
			if _, exists := desired.Annotations[k]; !exists {
				desired.Annotations[k] = v
			}
		}
	}

	if err := controllerutil.SetControllerReference(cluster, desired, scheme); err != nil {
		return false, fmt.Errorf("set owner ref on StatefulSet: %w", err)
	}

	changed, err := applyResourceSSA(ctx, kc, desired, "ravendb-operator/statefulset")

	if err != nil {
		return false, fmt.Errorf("failed to apply StatefulSet: %w", err)
	}

	return changed, nil
}
