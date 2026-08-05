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
	"testing"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRunMigratesLegacyPodTemplatesOneNodePerTickBeforeBootstrap(t *testing.T) {
	t.Parallel()

	const image = "ravendb/ravendb:7.1.3-ubuntu.22.04-x64"
	cluster := testCluster(image, "A", "B")
	kc := fakeClient(t,
		legacyStatefulSet(cluster.Namespace, "A", image),
		legacyStatefulSet(cluster.Namespace, "B", image),
	)

	u := NewUpgrader(Timing{}).(*upgrader)
	u.buildGates = func(context.Context, client.Client, *ravendbv1.RavenDBCluster) (*HealthCheckContext, error) {
		return nil, fmt.Errorf("HTTP gates must not be built before bootstrap")
	}

	var applied []string
	applyNode := func(node ravendbv1.RavenDBNode) error {
		var sts appsv1.StatefulSet
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: common.NodeResourceName(node.Tag)}
		if err := kc.Get(context.Background(), key, &sts); err != nil {
			return err
		}
		if sts.Spec.Template.Annotations == nil {
			sts.Spec.Template.Annotations = map[string]string{}
		}
		sts.Spec.Template.Annotations[common.PodTemplateRevisionAnnotation] = common.CurrentPodTemplateRevision
		if err := kc.Update(context.Background(), &sts); err != nil {
			return err
		}
		applied = append(applied, node.Tag)
		return nil
	}

	_, err := u.Run(context.Background(), cluster, kc, applyNode)
	require.NoError(t, err)
	require.Equal(t, []string{"A"}, applied)
	require.Equal(t, common.CurrentPodTemplateRevision, loadRevision(t, kc, cluster.Namespace, "A"))
	require.Empty(t, loadRevision(t, kc, cluster.Namespace, "B"))

	_, err = u.Run(context.Background(), cluster, kc, applyNode)
	require.NoError(t, err)
	require.Equal(t, []string{"A", "B"}, applied)
	require.Equal(t, common.CurrentPodTemplateRevision, loadRevision(t, kc, cluster.Namespace, "B"))
}

func TestRunLeavesHealthyLegacyPodTemplatesUntouched(t *testing.T) {
	t.Parallel()

	const image = "ravendb/ravendb:7.1.3-ubuntu.22.04-x64"
	cluster := testCluster(image, "A", "B")
	cluster.SetBootstrapped(metav1.Now())
	kc := fakeClient(t,
		legacyStatefulSet(cluster.Namespace, "A", image),
		legacyStatefulSet(cluster.Namespace, "B", image),
	)

	u := NewUpgrader(Timing{}).(*upgrader)
	applies := 0
	_, err := u.Run(context.Background(), cluster, kc, func(ravendbv1.RavenDBNode) error {
		applies++
		return nil
	})

	require.NoError(t, err)
	require.Zero(t, applies)
	require.Empty(t, loadRevision(t, kc, cluster.Namespace, "A"))
	require.Empty(t, loadRevision(t, kc, cluster.Namespace, "B"))
}

func TestPickSelectedTagCarriesRevisionWithIntentionalImageUpgrade(t *testing.T) {
	t.Parallel()

	cluster := testCluster("image:v1", "A")
	cluster.SetBootstrapped(metav1.Now())
	kc := fakeClient(t, legacyStatefulSet(cluster.Namespace, "A", "image:v1"))
	u := NewUpgrader(Timing{}).(*upgrader)

	tag, err := u.pickSelectedTag(context.Background(), kc, cluster, "image:v1")
	require.NoError(t, err)
	require.Empty(t, tag)

	tag, err = u.pickSelectedTag(context.Background(), kc, cluster, "image:v2")
	require.NoError(t, err)
	require.Equal(t, "A", tag)
}

func testCluster(image string, tags ...string) *ravendbv1.RavenDBCluster {
	nodes := make([]ravendbv1.RavenDBNode, 0, len(tags))
	for _, tag := range tags {
		nodes = append(nodes, ravendbv1.RavenDBNode{Tag: tag})
	}
	return &ravendbv1.RavenDBCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "test"},
		Spec: ravendbv1.RavenDBClusterSpec{
			Image: image,
			Nodes: nodes,
		},
	}
}

func fakeClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func legacyStatefulSet(namespace, tag, image string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NodeResourceName(tag),
			Namespace: namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "ravendb", Image: image}},
				},
			},
		},
	}
}

func loadRevision(t *testing.T, kc client.Client, namespace, tag string) string {
	t.Helper()
	var sts appsv1.StatefulSet
	require.NoError(t, kc.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      common.NodeResourceName(tag),
	}, &sts))
	return podTemplateRevision(&sts)
}
