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
	"sync/atomic"
	"testing"
	"time"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestRunDoesNotUsePostHTTPGatesBeforeReplacementPod(t *testing.T) {
	t.Parallel()

	const (
		currentImage = "image:v1"
		desiredImage = "image:v2"
	)

	var applied atomic.Bool
	var postHTTPCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if applied.Load() {
			postHTTPCalls.Add(1)
		}
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

	cluster := transactionCluster(desiredImage, "A")
	cluster.Spec.Nodes[0].PublicServerUrl = server.URL
	cluster.SetBootstrapped(metav1.Now())
	kc := transactionClient(t,
		transactionStatefulSet(cluster.Namespace, "A", currentImage, ""),
		transactionPod(cluster.Namespace, "A", currentImage),
	)

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

	_, err := u.Run(context.Background(), cluster, kc, func(node ravendbv1.RavenDBNode) error {
		var sts appsv1.StatefulSet
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: common.NodeResourceName(node.Tag)}
		require.NoError(t, kc.Get(context.Background(), key, &sts))
		sts.Spec.Template.Spec.Containers[0].Image = desiredImage
		require.NoError(t, kc.Update(context.Background(), &sts))
		applied.Store(true)
		return nil
	})

	require.ErrorContains(t, err, "observe replacement Pod for A")
	require.Zero(t, postHTTPCalls.Load())
	require.Equal(t, desiredImage, rolloutMarker(t, kc, cluster.Namespace, "A"))
}
