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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRunRecreatesOnlySelectedPodWhenInFlightTargetChanges(t *testing.T) {
	t.Parallel()

	const (
		failedImage   = "image:missing"
		repairedImage = "image:v2"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/setup/alive":
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

	cluster := transactionCluster(repairedImage, "A", "B")
	cluster.Spec.Nodes[0].PublicServerUrl = server.URL
	cluster.SetBootstrapped(metav1.Now())

	stsA := transactionStatefulSet(cluster.Namespace, "A", failedImage, failedImage)
	stsB := transactionStatefulSet(cluster.Namespace, "B", repairedImage, "")
	podA := transactionPod(cluster.Namespace, "A", failedImage)
	podA.UID = types.UID("pod-a")
	podB := transactionPod(cluster.Namespace, "B", repairedImage)
	podB.UID = types.UID("pod-b")
	kc := transactionClient(t, stsA, stsB, podA, podB)

	u := NewUpgrader(Timing{
		PreMaxWait:      50 * time.Millisecond,
		PostMaxWait:     2 * time.Millisecond,
		PingInterval:    time.Millisecond,
		DBInterval:      time.Millisecond,
		GraceAfterReady: time.Nanosecond,
	}).(*upgrader)
	u.buildGates = func(context.Context, client.Client, *ravendbv1.RavenDBCluster) (*HealthCheckContext, error) {
		return NewChecks(server.Client(), cluster), nil
	}
	applyNode := func(node ravendbv1.RavenDBNode) error {
		var sts appsv1.StatefulSet
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: common.NodeResourceName(node.Tag)}
		require.NoError(t, kc.Get(context.Background(), key, &sts))
		sts.Spec.Template.Spec.Containers[0].Image = repairedImage
		return kc.Update(context.Background(), &sts)
	}

	_, err := u.Run(context.Background(), cluster, kc, applyNode)
	require.ErrorContains(t, err, "observe replacement Pod for A")

	var deleted corev1.Pod
	err = kc.Get(context.Background(), client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      common.NodeResourceName("A") + "-0",
	}, &deleted)
	require.True(t, kerrors.IsNotFound(err))

	var untouched corev1.Pod
	require.NoError(t, kc.Get(context.Background(), client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      common.NodeResourceName("B") + "-0",
	}, &untouched))
	require.Equal(t, types.UID("pod-b"), untouched.UID)
	require.Equal(t, failedImage, rolloutMarker(t, kc, cluster.Namespace, "A"))

	repairedPod := transactionPod(cluster.Namespace, "A", repairedImage)
	repairedPod.UID = types.UID("pod-a-repaired")
	require.NoError(t, kc.Create(context.Background(), repairedPod))

	_, err = u.Run(context.Background(), cluster, kc, applyNode)
	require.NoError(t, err)
	require.Empty(t, rolloutMarker(t, kc, cluster.Namespace, "A"))
}
