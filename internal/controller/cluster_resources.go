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

package controller

// Shared by the controllers of resources that live inside a RavenDBCluster: RavenDBDatabase and
// RavenDBClientCertificate. Their status follows kstatus (observedGeneration plus the Reconciling and
// Stalled conditions), so `helm --wait` can wait for them like for any built-in resource.

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/ravendb"
)

const (
	conditionReady       = "Ready"
	conditionReconciling = "Reconciling"
	conditionStalled     = "Stalled"
)

const (
	reasonReconciled      = "Reconciled"
	reasonReconcileError  = "ReconcileError"
	reasonClusterNotFound = "ClusterNotFound"
	reasonClusterNotReady = "ClusterNotReady"
	reasonRejected        = "RejectedByRavenDB"
	reasonSecretConflict  = "SecretConflict"
	reasonSecretRejected  = "SecretRejected"
)

const (
	cleanupFinalizer = "ravendb.ravendb.io/cleanup"
	retryInterval    = 15 * time.Second
	// resyncInterval bounds how long a change made directly in RavenDB (a revoked certificate, a deleted
	// database) goes unnoticed.
	resyncInterval = 10 * time.Minute
	// maxConcurrentReconciles keeps one unreachable cluster from holding up the resources of the others.
	maxConcurrentReconciles = 4
)

// ravenDB is the part of the RavenDB admin API the controllers use.
type ravenDB interface {
	DatabaseExists(ctx context.Context, name string) (bool, error)
	CreateDatabase(ctx context.Context, name string, replicationFactor int32) error
	DeleteDatabase(ctx context.Context, name string) error
	Certificates(ctx context.Context) ([]ravendb.Certificate, error)
	RegisterCertificate(ctx context.Context, cert ravendb.Certificate) error
	EditCertificate(ctx context.Context, cert ravendb.Certificate) error
	RevokeCertificate(ctx context.Context, thumbprint string) error
}

type connectFunc func(context.Context, client.Client, *ravendbv1.RavenDBCluster) (ravenDB, error)

// connectWithAdminCertificate connects to the cluster's nodes with its admin client certificate.
func connectWithAdminCertificate(ctx context.Context, kc client.Client, c *ravendbv1.RavenDBCluster) (ravenDB, error) {
	return ravendb.ForCluster(ctx, kc, c)
}

// notReady explains why a resource is not reconciled yet. A stalled resource needs someone to change
// something; any other one is retried soon.
type notReady struct {
	reason  string
	message string
	stalled bool
}

func (e *notReady) Error() string { return e.message }

func waiting(reason, format string, args ...any) error {
	return &notReady{reason: reason, message: fmt.Sprintf(format, args...)}
}

func stalled(reason, format string, args ...any) error {
	return &notReady{reason: reason, message: fmt.Sprintf(format, args...), stalled: true}
}

// connect returns the referenced cluster and an admin client for it, once the cluster has bootstrapped.
func connect(
	ctx context.Context, kc client.Client, connectTo connectFunc, namespace string, ref ravendbv1.ClusterReference,
) (*ravendbv1.RavenDBCluster, ravenDB, error) {
	var cluster ravendbv1.RavenDBCluster
	err := kc.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ref.Name}, &cluster)
	if kerrors.IsNotFound(err) {
		return nil, nil, waiting(reasonClusterNotFound, "RavenDBCluster %q not found", ref.Name)
	}
	if err != nil {
		return nil, nil, err
	}
	if !cluster.IsBootstrapped() {
		return nil, nil, waiting(reasonClusterNotReady, "RavenDBCluster %q has not finished bootstrapping", ref.Name)
	}

	rc, err := connectTo(ctx, kc, &cluster)
	return &cluster, rc, err
}

// recordOutcome sets the conditions for the result of a reconcile and returns when to reconcile again.
func recordOutcome(conditions *[]metav1.Condition, generation int64, err error, readyMessage string) time.Duration {
	set := func(conditionType string, status metav1.ConditionStatus, reason, message string) {
		meta.SetStatusCondition(conditions, metav1.Condition{
			Type:               conditionType,
			Status:             status,
			Reason:             reason,
			Message:            message,
			ObservedGeneration: generation,
		})
	}

	if err == nil {
		set(conditionReady, metav1.ConditionTrue, reasonReconciled, readyMessage)
		meta.RemoveStatusCondition(conditions, conditionReconciling)
		meta.RemoveStatusCondition(conditions, conditionStalled)
		return resyncInterval
	}

	cause := explain(err)
	set(conditionReady, metav1.ConditionFalse, cause.reason, cause.message)
	if cause.stalled {
		set(conditionStalled, metav1.ConditionTrue, cause.reason, cause.message)
		meta.RemoveStatusCondition(conditions, conditionReconciling)
		return resyncInterval
	}
	set(conditionReconciling, metav1.ConditionTrue, cause.reason, cause.message)
	meta.RemoveStatusCondition(conditions, conditionStalled)
	return retryInterval
}

func explain(err error) *notReady {
	var nr *notReady
	if errors.As(err, &nr) {
		return nr
	}
	if ravendb.IsRejected(err) {
		return &notReady{reason: reasonRejected, message: err.Error(), stalled: true}
	}
	return &notReady{reason: reasonReconcileError, message: err.Error()}
}

// patchStatus writes the status when the reconcile changed it.
func patchStatus(ctx context.Context, kc client.Client, original, current client.Object) error {
	if equality.Semantic.DeepEqual(original, current) {
		return nil
	}
	return kc.Status().Patch(ctx, current, client.MergeFrom(original))
}

// addFinalizer makes sure deleting the resource waits for the cleanup in RavenDB.
func addFinalizer(ctx context.Context, kc client.Client, obj client.Object) error {
	if controllerutil.AddFinalizer(obj, cleanupFinalizer) {
		return kc.Update(ctx, obj)
	}
	return nil
}

func removeFinalizer(ctx context.Context, kc client.Client, obj client.Object) error {
	if controllerutil.RemoveFinalizer(obj, cleanupFinalizer) {
		return kc.Update(ctx, obj)
	}
	return nil
}

// connectForCleanup is connect for a resource being deleted. It returns no client when the cluster no longer
// exists, since nothing is left to clean up in it; a cluster that only is not bootstrapped yet is an error, so
// the deletion waits for it.
func connectForCleanup(
	ctx context.Context, kc client.Client, connectTo connectFunc, namespace string, ref ravendbv1.ClusterReference,
) (ravenDB, error) {
	_, rc, err := connect(ctx, kc, connectTo, namespace, ref)
	if err != nil && explain(err).reason == reasonClusterNotFound {
		return nil, nil
	}
	return rc, err
}

// cleanupBlocked reports why deleting obj waits, and returns err so the deletion is retried.
func cleanupBlocked(rec record.EventRecorder, obj client.Object, err error) error {
	rec.Eventf(obj, corev1.EventTypeWarning, "CleanupBlocked", "Waiting to clean up in RavenDB: %v", err)
	return err
}

// everyInNamespace requests a reconcile of every resource of the listed kind in the cluster's namespace, so they
// react to the cluster appearing or finishing its bootstrap. A namespace holds one cluster, so these are the
// resources that can depend on it.
func everyInNamespace(kc client.Client, newList func() client.ObjectList) handler.MapFunc {
	return func(ctx context.Context, cluster client.Object) []reconcile.Request {
		list := newList()
		if err := kc.List(ctx, list, client.InNamespace(cluster.GetNamespace())); err != nil {
			log.FromContext(ctx).Error(err, "list resources depending on RavenDBCluster", "cluster", cluster.GetName())
			return nil
		}
		items, err := meta.ExtractList(list)
		if err != nil {
			log.FromContext(ctx).Error(err, "read resources depending on RavenDBCluster", "cluster", cluster.GetName())
			return nil
		}
		requests := make([]reconcile.Request, 0, len(items))
		for _, item := range items {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(item.(client.Object))})
		}
		return requests
	}
}
