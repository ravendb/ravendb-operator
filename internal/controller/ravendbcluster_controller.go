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

	"ravendb-operator/pkg/common"
	"ravendb-operator/pkg/director"

	batchv1 "k8s.io/api/batch/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
)

// RavenDBClusterReconciler reconciles a RavenDBCluster object
type RavenDBClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Director director.Director
}

// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=ravendb.ravendb.io,resources=ravendbclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ravendb.ravendb.io,resources=ravendbclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ravendb.ravendb.io,resources=ravendbclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch

func (r *RavenDBClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var instance ravendbv1alpha1.RavenDBCluster
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if kerrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	d := r.Director

	if err := d.ExecutePerCluster(ctx, &instance, r.Client, r.Scheme); err != nil {
		l.Error(err, "failed to execute cluster-level actors")
		_ = r.setBootstrappedIfSucceeded(ctx, &instance)
		return ctrl.Result{}, err

	}

	// if the bootstrapper job completed, mark the CR as done.
	if err := r.setBootstrappedIfSucceeded(ctx, &instance); err != nil {
		l.Error(err, "failed to set Bootstrapped condition")
		return ctrl.Result{Requeue: true}, nil
	}

	var statuses []ravendbv1alpha1.RavenDBNodeStatus
	for _, node := range instance.Spec.Nodes {
		if err := d.ExecutePerNode(ctx, &instance, node, r.Client, r.Scheme); err != nil {
			l.Error(err, "failed to reconcile node", "node", node.Tag)
			statuses = append(statuses, ravendbv1alpha1.RavenDBNodeStatus{
				Tag:    node.Tag,
				Status: "Failed",
			})
			continue
		}

		statuses = append(statuses, ravendbv1alpha1.RavenDBNodeStatus{
			Tag:    node.Tag,
			Status: "Created",
		})
	}

	instance.Status.Nodes = statuses
	instance.Status.Phase = "Deploying"
	instance.Status.Message = fmt.Sprintf("Ensured desired state for %d RavenDB nodes", len(statuses))

	if err := r.Status().Update(ctx, &instance); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

// todo: to be moved in healt probe issue
func (r *RavenDBClusterReconciler) setBootstrappedIfSucceeded(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster) error {

	if cluster.IsBootstrapped() {
		return nil
	}

	const jobName = common.RavenDbBootstrapperJob

	var job batchv1.Job
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: cluster.Namespace,
		Name:      jobName,
	}, &job); err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if job.Status.Succeeded >= 1 {
		cluster.SetBootstrapped(metav1.Now())
		return r.Status().Update(ctx, cluster)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RavenDBClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ravendbv1alpha1.RavenDBCluster{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
