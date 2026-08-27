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

package upgrade

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"
)

// podImageApplied reports whether the node's Pod already runs desiredImage.
// Patching the StatefulSet does not stop the running RavenDB process: it keeps
// answering /setup/alive until Kubernetes swaps the Pod, so the HTTP gates
// would pass against the very image the upgrade is replacing.
func podImageApplied(
	ctx context.Context,
	kc client.Client,
	c *ravendbv1.RavenDBCluster,
	tag, desiredImage string,
) (bool, string, error) {
	var pod corev1.Pod
	key := client.ObjectKey{Namespace: c.Namespace, Name: common.NodePodName(tag)}
	if err := kc.Get(ctx, key, &pod); err != nil {
		if kerrors.IsNotFound(err) {
			return false, "replacement Pod has not been created", nil
		}
		return false, "", err
	}

	if image := firstContainerImage(pod.Spec.Containers); image != desiredImage {
		return false, fmt.Sprintf("observed image=%q, want image=%q", image, desiredImage), nil
	}
	return true, "", nil
}
