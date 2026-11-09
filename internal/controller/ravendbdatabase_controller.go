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

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"
)

// RavenDBDatabaseReconciler creates the database a RavenDBDatabase declares. Several resources may declare the
// same database; the Delete policy drops it with the last of them.
type RavenDBDatabaseReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Connect  connectFunc
}

// +kubebuilder:rbac:groups=ravendb.ravendb.io,resources=ravendbdatabases,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=ravendb.ravendb.io,resources=ravendbdatabases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ravendb.ravendb.io,resources=ravendbdatabases/finalizers,verbs=update
func (r *RavenDBDatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var db ravendbv1.RavenDBDatabase
	if err := r.Get(ctx, req.NamespacedName, &db); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !db.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, &db)
	}
	if err := addFinalizer(ctx, r.Client, &db); err != nil {
		return ctrl.Result{}, err
	}

	original := db.DeepCopy()
	err := r.ensureDatabase(ctx, &db)
	if err != nil {
		log.FromContext(ctx).Info("Database not ready", "cause", err.Error())
	}
	db.Status.ObservedGeneration = db.Generation
	requeueAfter := recordOutcome(&db.Status.Conditions, db.Generation, err,
		fmt.Sprintf("Database %q exists", db.Spec.DatabaseName))

	return ctrl.Result{RequeueAfter: requeueAfter}, patchStatus(ctx, r.Client, original, &db)
}

func (r *RavenDBDatabaseReconciler) ensureDatabase(ctx context.Context, db *ravendbv1.RavenDBDatabase) error {
	_, rc, err := connect(ctx, r.Client, r.Connect, db.Namespace, db.Spec.ClusterRef)
	if err != nil {
		return err
	}
	exists, err := rc.DatabaseExists(ctx, db.Spec.DatabaseName)
	if err != nil || exists {
		return err
	}

	if err := rc.CreateDatabase(ctx, db.Spec.DatabaseName, db.Spec.ReplicationFactor); err != nil {
		return err
	}
	r.Recorder.Eventf(db, corev1.EventTypeNormal, "DatabaseCreated",
		"Created database %q with replication factor %d", db.Spec.DatabaseName, db.Spec.ReplicationFactor)
	return nil
}

// finalize deletes the database when the policy says so, then lets Kubernetes remove the resource.
func (r *RavenDBDatabaseReconciler) finalize(ctx context.Context, db *ravendbv1.RavenDBDatabase) error {
	if db.Spec.DeletionPolicy == ravendbv1.DeletionPolicyDelete {
		if err := r.deleteDatabase(ctx, db); err != nil {
			return cleanupBlocked(r.Recorder, db, err)
		}
	}
	return removeFinalizer(ctx, r.Client, db)
}

// deleteDatabase drops the database and its data, unless another resource still declares it.
func (r *RavenDBDatabaseReconciler) deleteDatabase(ctx context.Context, db *ravendbv1.RavenDBDatabase) error {
	shared, err := r.declaredElsewhere(ctx, db)
	if err != nil || shared {
		return err
	}
	rc, err := connectForCleanup(ctx, r.Client, r.Connect, db.Namespace, db.Spec.ClusterRef)
	if err != nil || rc == nil {
		return err
	}
	exists, err := rc.DatabaseExists(ctx, db.Spec.DatabaseName)
	if err != nil || !exists {
		return err
	}

	if err := rc.DeleteDatabase(ctx, db.Spec.DatabaseName); err != nil {
		return err
	}
	r.Recorder.Eventf(db, corev1.EventTypeNormal, "DatabaseDeleted", "Deleted database %q", db.Spec.DatabaseName)
	return nil
}

// declaredElsewhere reports whether another live resource declares the same database. RavenDB database names are
// case-insensitive.
func (r *RavenDBDatabaseReconciler) declaredElsewhere(ctx context.Context, db *ravendbv1.RavenDBDatabase) (bool, error) {
	var list ravendbv1.RavenDBDatabaseList
	if err := r.List(ctx, &list, client.InNamespace(db.Namespace)); err != nil {
		return false, err
	}
	for _, other := range list.Items {
		if other.Name != db.Name && other.DeletionTimestamp.IsZero() &&
			other.Spec.ClusterRef == db.Spec.ClusterRef && strings.EqualFold(other.Spec.DatabaseName, db.Spec.DatabaseName) {
			return true, nil
		}
	}
	return false, nil
}

func (r *RavenDBDatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor(common.Manager)
	if r.Connect == nil {
		r.Connect = connectWithAdminCertificate
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&ravendbv1.RavenDBDatabase{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&ravendbv1.RavenDBCluster{}, handler.EnqueueRequestsFromMapFunc(
			everyInNamespace(r.Client, func() client.ObjectList { return &ravendbv1.RavenDBDatabaseList{} }))).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}
