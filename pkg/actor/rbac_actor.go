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

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type RBACActor struct{}

func NewRBACActor() PerClusterActor {
	return &RBACActor{}
}

func (a *RBACActor) Name() string {
	return "RBACActor"
}

func (a *RBACActor) ShouldAct(cluster *ravendbv1.RavenDBCluster) bool {
	return true
}

func (a *RBACActor) Act(
	ctx context.Context,
	cluster *ravendbv1.RavenDBCluster,
	c client.Client,
	scheme *runtime.Scheme,
) (bool, error) {
	_ = scheme

	objs := resource.BuildRavenDBOpsRBAC(cluster)

	changedAny := false
	for _, obj := range objs {
		if err := controllerutil.SetControllerReference(cluster, obj, scheme); err != nil {
			return false, fmt.Errorf("set controller reference for %T: %w", obj, err)
		}

		changed, err := applyResourceSSA(ctx, c, obj, "ravendb-operator/rbac")
		if err != nil {
			return false, fmt.Errorf("apply %T: %w", obj, err)
		}
		if changed {
			changedAny = true
		}
	}

	return changedAny, nil
}
