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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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

func TestRunKeepsMarkerUntilPostGatesRecover(t *testing.T) {
	t.Parallel()

	const (
		currentImage = "image:v1"
		desiredImage = "image:v2"
	)

	var healthy atomic.Bool
	healthy.Store(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/setup/alive":
			if !healthy.Load() {
				http.Error(w, "node is restarting", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		case "/admin/debug/node/ping":
			_, _ = w.Write([]byte(`{"Result":[{"Url":"http://node-a","SetupAlive":{"Error":""},"TcpInfo":{"Error":""}}]}`))
		case "/databases":
			_, _ = w.Write([]byte(`{"Databases":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cluster := transactionCluster(desiredImage, "A")
	cluster.Spec.Nodes[0].PublicServerUrl = server.URL
	cluster.SetBootstrapped(metav1.Now())
	kc := transactionClient(t,
		transactionStatefulSet(cluster.Namespace, "A", currentImage, ""),
		transactionPod(cluster.Namespace, "A", currentImage),
	)

	u := NewUpgrader(Timing{
		PreMaxWait:      50 * time.Millisecond,
		PostMaxWait:     time.Millisecond,
		PingInterval:    time.Millisecond,
		DBInterval:      time.Millisecond,
		GraceAfterReady: time.Nanosecond,
	}).(*upgrader)
	u.buildGates = func(context.Context, client.Client, *ravendbv1.RavenDBCluster) (*HealthCheckContext, error) {
		return NewChecks(server.Client(), cluster), nil
	}

	sabotageAfterFirstApply := true
	applies := 0
	applyNode := func(node ravendbv1.RavenDBNode) error {
		applies++
		var sts appsv1.StatefulSet
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: common.NodeResourceName(node.Tag)}
		require.NoError(t, kc.Get(context.Background(), key, &sts))
		sts.Spec.Template.Spec.Containers[0].Image = desiredImage
		require.NoError(t, kc.Update(context.Background(), &sts))
		var pod corev1.Pod
		require.NoError(t, kc.Get(context.Background(), client.ObjectKey{
			Namespace: cluster.Namespace,
			Name:      common.NodeResourceName(node.Tag) + "-0",
		}, &pod))
		pod.Spec.Containers[0].Image = desiredImage
		require.NoError(t, kc.Update(context.Background(), &pod))
		if sabotageAfterFirstApply {
			sabotageAfterFirstApply = false
			healthy.Store(false)
		}
		return nil
	}

	_, err := u.Run(context.Background(), cluster, kc, applyNode)
	require.ErrorContains(t, err, "post-node gates failed for A")
	require.Equal(t, desiredImage, rolloutMarker(t, kc, cluster.Namespace, "A"))

	healthy.Store(true)
	_, err = u.Run(context.Background(), cluster, kc, applyNode)
	require.NoError(t, err)
	require.Empty(t, rolloutMarker(t, kc, cluster.Namespace, "A"))
	require.Equal(t, 2, applies)
}

func TestRunKeepsMarkerAndAllStatusesWhenApplyFails(t *testing.T) {
	t.Parallel()

	const desiredImage = "image:v2"
	cluster := transactionCluster(desiredImage, "A", "B")
	kc := transactionClient(t,
		transactionStatefulSet(cluster.Namespace, "A", "image:v1", desiredImage),
		transactionStatefulSet(cluster.Namespace, "B", "image:v1", ""),
	)

	u := NewUpgrader(Timing{}).(*upgrader)
	u.buildGates = func(context.Context, client.Client, *ravendbv1.RavenDBCluster) (*HealthCheckContext, error) {
		return &HealthCheckContext{}, nil
	}

	statuses, err := u.Run(context.Background(), cluster, kc, func(ravendbv1.RavenDBNode) error {
		return fmt.Errorf("synthetic apply failure")
	})

	require.ErrorContains(t, err, "synthetic apply failure")
	require.Equal(t, desiredImage, rolloutMarker(t, kc, cluster.Namespace, "A"))
	require.Len(t, statuses, 2)
	require.Equal(t, "A", statuses[0].Tag)
	require.Equal(t, ravendbv1.NodeStatusFailed, statuses[0].Status)
	require.Equal(t, "B", statuses[1].Tag)
	require.Equal(t, ravendbv1.NodeStatusCreated, statuses[1].Status)
}

func TestMarkerPatchDoesNotReadThroughCache(t *testing.T) {
	t.Parallel()

	cluster := transactionCluster("image:v2", "A")
	base := transactionClient(t, transactionStatefulSet(cluster.Namespace, "A", "image:v1", ""))
	kc := &failOnGetClient{Client: base}
	u := NewUpgrader(Timing{}).(*upgrader)

	require.NoError(t, u.setUpgradeAnnotation(context.Background(), kc, cluster, "A", "image:v2"))
	require.Equal(t, "image:v2", rolloutMarker(t, base, cluster.Namespace, "A"))

	require.NoError(t, u.setUpgradeAnnotation(context.Background(), kc, cluster, "A", ""))
	require.Empty(t, rolloutMarker(t, base, cluster.Namespace, "A"))
	require.Zero(t, kc.gets)
}

func TestWaitStopsDuringBackoffWhenContextIsCancelled(t *testing.T) {
	t.Parallel()

	u := NewUpgrader(Timing{PreMaxWait: time.Minute}).(*upgrader)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(10*time.Millisecond, cancel)

	start := time.Now()
	err := u.wait(ctx, transactionCluster("image:v1", "A"), GatePreStep, GateNodeAlive, "A", time.Minute,
		func() (bool, string, error) { return false, "not ready", nil })

	require.Error(t, err)
	require.Less(t, time.Since(start), time.Second)
}

type failOnGetClient struct {
	client.Client
	gets int
}

func (c *failOnGetClient) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	c.gets++
	return fmt.Errorf("unexpected cached read")
}

func transactionCluster(image string, tags ...string) *ravendbv1.RavenDBCluster {
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

func transactionClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func transactionStatefulSet(namespace, tag, image, marker string) *appsv1.StatefulSet {
	annotations := map[string]string{}
	if marker != "" {
		annotations[common.UpgradeImageAnnotation] = marker
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        common.NodeResourceName(tag),
			Namespace:   namespace,
			Annotations: annotations,
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

func transactionPod(namespace, tag, image string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.NodeResourceName(tag) + "-0",
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "ravendb", Image: image}},
		},
	}
}

func rolloutMarker(t *testing.T, kc client.Client, namespace, tag string) string {
	t.Helper()
	var sts appsv1.StatefulSet
	require.NoError(t, kc.Get(context.Background(), client.ObjectKey{
		Namespace: namespace,
		Name:      common.NodeResourceName(tag),
	}, &sts))
	return sts.Annotations[common.UpgradeImageAnnotation]
}
