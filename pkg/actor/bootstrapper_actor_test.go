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
	"testing"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestBootstrapperJobReplacesOnlyTerminalFailedOldRevision(t *testing.T) {
	tests := []struct {
		name        string
		current     bool
		status      batchv1.JobStatus
		wantDeleted bool
	}{
		{
			name:   "active old revision is allowed to finish",
			status: batchv1.JobStatus{Active: 1},
		},
		{
			name: "completed old revision is retained",
			status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
				Type: batchv1.JobComplete, Status: corev1.ConditionTrue,
			}}},
		},
		{
			name: "terminal failed old revision is deleted",
			status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
				Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
			}}},
			wantDeleted: true,
		},
		{
			name:    "terminal failed current revision is retained",
			current: true,
			status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
				Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
			}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, ravendbv1.AddToScheme(scheme))
			require.NoError(t, batchv1.AddToScheme(scheme))

			cluster := &ravendbv1.RavenDBCluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: ravendbv1.GroupVersion.String(),
					Kind:       "RavenDBCluster",
				},
				ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "test", UID: "cluster-uid"},
			}
			desired := bootstrapJob(common.CurrentPodTemplateRevision)
			existing := bootstrapJob("")
			existing.Status = tt.status
			if tt.current {
				existing.Spec.Template.Annotations[common.PodTemplateRevisionAnnotation] = common.CurrentPodTemplateRevision
			}

			var deleteOpts []client.DeleteOption
			kc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
						deleteOpts = opts
						return c.Delete(ctx, obj, opts...)
					},
				}).Build()
			actor := NewBootstrapperActor(staticClusterBuilder{job: desired})
			_, err := actor.Act(context.Background(), cluster, kc, scheme)
			require.NoError(t, err)

			var got batchv1.Job
			err = kc.Get(context.Background(), client.ObjectKeyFromObject(existing), &got)
			if tt.wantDeleted {
				require.True(t, kerrors.IsNotFound(err))

				var gotOpts client.DeleteOptions
				gotOpts.ApplyOptions(deleteOpts)
				require.NotNil(t, gotOpts.PropagationPolicy, "delete must cascade to the failed pod")
				require.Equal(t, metav1.DeletePropagationBackground, *gotOpts.PropagationPolicy)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

type staticClusterBuilder struct {
	job *batchv1.Job
}

func (b staticClusterBuilder) Build(context.Context, *ravendbv1.RavenDBCluster) (client.Object, error) {
	return b.job.DeepCopy(), nil
}

func bootstrapJob(revision string) *batchv1.Job {
	annotations := map[string]string{}
	if revision != "" {
		annotations[common.PodTemplateRevisionAnnotation] = revision
	}

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.RavenDbBootstrapperJob,
			Namespace: "test",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Annotations: annotations},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers:    []corev1.Container{{Name: "bootstrapper", Image: "image:v1"}},
				},
			},
		},
	}
}
