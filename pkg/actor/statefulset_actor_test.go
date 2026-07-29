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

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestStatefulSetActorAppliesSelectedImageWithoutMarkerCacheRead(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, ravendbv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	cluster := &ravendbv1.RavenDBCluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ravendbv1.GroupVersion.String(),
			Kind:       "RavenDBCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db",
			Namespace: "test",
			UID:       "cluster-uid",
		},
	}
	existing := actorStatefulSet("old:image")
	existing.Annotations = map[string]string{"example.com/preserved": "true"}
	desired := actorStatefulSet("new:image")

	kc := &recordingPatchClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build(),
	}
	statefulSetActor := NewStatefulSetActor(staticNodeBuilder{object: desired})

	_, err := statefulSetActor.Act(
		context.Background(),
		cluster,
		ravendbv1.RavenDBNode{Tag: "A"},
		kc,
		scheme,
	)
	require.NoError(t, err)
	require.NotNil(t, kc.statefulSet)
	require.Equal(t, "new:image", kc.statefulSet.Spec.Template.Spec.Containers[0].Image,
		"the Upgrader already selected this node; applying it must not depend on cache visibility of the marker")
	require.Equal(t, "true", kc.statefulSet.Annotations["example.com/preserved"])
}

type recordingPatchClient struct {
	client.Client
	statefulSet *appsv1.StatefulSet
}

func (c *recordingPatchClient) Patch(
	_ context.Context,
	object client.Object,
	_ client.Patch,
	_ ...client.PatchOption,
) error {
	c.statefulSet = object.(*appsv1.StatefulSet).DeepCopy()
	return nil
}

type staticNodeBuilder struct {
	object client.Object
}

func (b staticNodeBuilder) Build(
	context.Context,
	*ravendbv1.RavenDBCluster,
	ravendbv1.RavenDBNode,
) (client.Object, error) {
	return b.object.DeepCopyObject().(client.Object), nil
}

func actorStatefulSet(image string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ravendb-a",
			Namespace: "test",
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"node": "A"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node": "A"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "ravendb", Image: image}},
				},
			},
		},
	}
}
