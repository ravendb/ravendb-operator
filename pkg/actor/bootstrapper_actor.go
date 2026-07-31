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
	"ravendb-operator/pkg/common"
	"ravendb-operator/pkg/resource"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (actor *BootstrapperActor) Act(ctx context.Context, cluster *ravendbv1.RavenDBCluster, kc client.Client, scheme *runtime.Scheme) (bool, error) {
	bs, err := actor.builder.Build(ctx, cluster)
	if err != nil {
		return false, fmt.Errorf("failed to build bootstrapper resource: %w", err)
	}

	if err := controllerutil.SetControllerReference(cluster, bs, scheme); err != nil {
		return false, fmt.Errorf("set owner ref on bootstrapper resource: %w", err)
	}

	if desired, ok := bs.(*batchv1.Job); ok {
		return actor.reconcileJob(ctx, kc, desired)
	}

	_, err = applyResourceSSA(ctx, kc, bs, "ravendb-operator/cluster")
	return false, err
}

func (actor *BootstrapperActor) ShouldAct(cluster *ravendbv1.RavenDBCluster) bool {
	return !cluster.IsBootstrapped()
}

// Failed legacy Jobs must be recreated because Job pod templates are immutable.
func (actor *BootstrapperActor) reconcileJob(ctx context.Context, kc client.Client, desired *batchv1.Job) (bool, error) {
	var existing batchv1.Job
	key := client.ObjectKeyFromObject(desired)
	if err := kc.Get(ctx, key, &existing); err != nil {
		if kerrors.IsNotFound(err) {
			_, applyErr := applyResourceSSA(ctx, kc, desired, "ravendb-operator/job")
			return false, applyErr
		}
		return false, fmt.Errorf("get bootstrapper Job %s/%s: %w", key.Namespace, key.Name, err)
	}

	if podTemplateRevision(existing.Spec.Template.Annotations) == podTemplateRevision(desired.Spec.Template.Annotations) {
		return false, nil
	}
	if !jobConditionTrue(&existing, batchv1.JobFailed) || existing.DeletionTimestamp != nil {
		return false, nil
	}

	// Jobs default to orphan propagation, which would leave the failed pod running.
	if err := kc.Delete(ctx, &existing, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !kerrors.IsNotFound(err) {
		return false, fmt.Errorf("delete failed old-revision bootstrapper Job %s/%s: %w", key.Namespace, key.Name, err)
	}
	return false, nil
}

func podTemplateRevision(annotations map[string]string) string {
	if annotations == nil {
		return ""
	}
	return annotations[common.PodTemplateRevisionAnnotation]
}

func jobConditionTrue(job *batchv1.Job, conditionType batchv1.JobConditionType) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == conditionType && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
