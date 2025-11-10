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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func applyResourceSSA(ctx context.Context, c client.Client, desired client.Object, fieldOwner string) (bool, error) {
	key := client.ObjectKey{Namespace: desired.GetNamespace(), Name: desired.GetName()}
	pre := desired.DeepCopyObject().(client.Object)
	preRV := ""
	if err := c.Get(ctx, key, pre); err == nil {
		preRV = pre.GetResourceVersion()
	} else if !apierrors.IsNotFound(err) {
		return false, fmt.Errorf("get before apply %T %s/%s: %w", desired, key.Namespace, key.Name, err)
	}
	desired.SetResourceVersion("")
	if err := c.Patch(ctx, desired, client.Apply, client.FieldOwner(fieldOwner), client.ForceOwnership); err != nil {
		return false, fmt.Errorf("apply (SSA) %T %s/%s: %w", desired, key.Namespace, key.Name, err)
	}
	if preRV == "" {
		return true, nil // created
	}
	return desired.GetResourceVersion() != preRV, nil
}
