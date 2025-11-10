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
	"reflect"

	"ravendb-operator/pkg/common"
	"ravendb-operator/pkg/director"
	"ravendb-operator/pkg/upgrade"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"ravendb-operator/pkg/health"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

/*
The RavenDBCluster Reconciliation flow

Top overview
---
The reconciler drives the desired cluster state by:
1) loading the RavenDBCluster,
2) delegating object creation/update to "actors" via a Director,
3) collecting live cluster facts,
4) evaluating health -> conditions -> phase,
5) persisting Status with conflict-safe merges
6) emitting Events for condition transitions.

Our goal is keep the real cluster matching the RavenDBCluster spec. each reconcile is:
"Look at the specs -> create/adjust K8s objects accordingly -> check what's running -> update status."


Cast
---
- RavenDBCluster: the single source of truth for desired state.
- Director: a coordinator. It tells actors what to create/update.
- Actors: small workers that own one thing (StatefulSet, Service, Ingress, Job).
- Health collector + evaluator: responsiable to ask "what's actually happening?" based on the answer -> compute conditions and phase.


Step by step (detailed flow)
---
1) load + snapshot
   - read the RavenDBCluster.
   - If it doesn't exist: we are done (CR deleted !!).
   - else: Keep a copy of the previous Status/Conditions so we can detect changes and emit events.

2) build + apply desired objects (via director + actors)
  - The director runs:
       - per cluster actors (e.g ingress) when they should act.
       - per node actors (e.g sts) once for each node in the spec.
   - each actor asks its builder to produce the desired K8s object.
   - we set an OwnerReference from every child object back to this RavenDBCluster.
   (meaning: 1. if the CR is deleted, K8s automatically cleans up our owned children
             2. when the collector look at the cluster later (status purposes), it can easily filter our objs)

   we apply resources using SSA:
   - SSA is a smart merge done by the K8s API https://kubernetes.io/docs/reference/using-api/server-side-apply/ .
   - we only own the fields we set. other controllers/users can manage other fields.
   - if there's a clash on a field we own, our controller wins (with ForceOwnership).
   - this avoids the "last write wins" problem and reduces conflicts.

3) observe reality
   - the collector lists what's in the cluster that we own (StatefulSets, Jobs, Services,
     Ingresses, Pods, PVCs) plus relevant Secrets.
   - it translates raw K8s objects into simple "facts" (names, phases, ready flags, etc.).

4) work out health and phase
   - the evaluator looks at the facts and sets conditions like:
     StorageReady, CertificatesReady, LicensesValid, NodesHealthy, ExternalAccessReady
     (if configured), BootstrapCompleted, Progressing, Degraded.
   - then we roll them up into a single Phase
       Ready -> Running
       else if Degraded -> Error
       else if Progressing -> Deploying
       else -> Deploying.

5) persist status with conflict handling
   compare original.Status vs instance.Status (DeepEqual).
   - if Status changed, we patch only the Status subresource against our earlier snapshot.
   - If the API says "conflict" (someone updated Status at the same time), we requeue and try again, instead of overwriting.

6) emit events on changes
   - if any condition's Status/Reason/Message changed, we log it and publish a K8s Event.

*/

// RavenDBClusterReconciler reconciles a RavenDBCluster object
type RavenDBClusterReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Director   director.Director
	Upgrader   upgrade.Upgrader
	Recorder   record.EventRecorder
	BaseTiming upgrade.Timing
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
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch;update

func (r *RavenDBClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var instance ravendbv1alpha1.RavenDBCluster
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if kerrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	original := instance.DeepCopy()
	prevConditions := append([]metav1.Condition(nil), original.Status.Conditions...)

	_, err := r.Director.ExecutePerCluster(ctx, &instance, r.Client, r.Scheme)
	if err != nil {
		logger.Error(err, "failed to execute cluster-level actors")
		return ctrl.Result{}, err
	}

	applyNode := func(node ravendbv1alpha1.RavenDBNode) error {
		_, err := r.Director.ExecutePerNode(ctx, &instance, node, r.Client, r.Scheme)
		return err
	}

	r.Upgrader.SetTiming(upgrade.ReadTimingFromAnnotations(&instance, r.BaseTiming))

	nodeStatuses, err := r.Upgrader.Run(ctx, &instance, r.Client, applyNode)
	if err != nil {
		logger.Error(err, "rolling upgrade failed")
		if r.Recorder != nil {
			r.Recorder.Eventf(&instance, corev1.EventTypeWarning, "RollingUpgradeFailed", "%v", err)
		}
	}
	instance.Status.Nodes = nodeStatuses

	resFacts, err := health.NewResourceCollector().Collect(ctx, r.Client, &instance)
	if err != nil {
		logger.Error(err, "resource translation failed")
	}
	ev := health.NewEvaluator()
	ev.Evaluate(ctx, &instance, resFacts, metav1.Now())

	statusChanged := !reflect.DeepEqual(original.Status, instance.Status)
	if statusChanged {
		if err := r.Status().Patch(ctx, &instance, client.MergeFrom(original)); err != nil {
			if kerrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		emitConditionTransitions(&instance, prevConditions, logger, r.Recorder)
	}

	return ctrl.Result{}, nil
}

func emitConditionTransitions(cluster *ravendbv1alpha1.RavenDBCluster, prevConditions []metav1.Condition, logger logr.Logger, rec record.EventRecorder) {

	previousByType := make(map[string]metav1.Condition, len(prevConditions))
	for i := 0; i < len(prevConditions); i++ {
		c := prevConditions[i]
		previousByType[c.Type] = c
	}

	for i := 0; i < len(cluster.Status.Conditions); i++ {

		cur := cluster.Status.Conditions[i]
		previous, hadPrevious := previousByType[cur.Type]

		if !hadPrevious || previous.Status != cur.Status || previous.Reason != cur.Reason || previous.Message != cur.Message {
			logger.Info("Condition transition", "condition", cur)
			eventType := getEventSeverity(cur)

			rec.Eventf(
				cluster,
				eventType,
				cur.Reason,
				"Condition %s changed to %s (reason=%s): %s",
				cur.Type, cur.Status, cur.Reason, cur.Message,
			)
		}
	}
}

func getEventSeverity(cur metav1.Condition) string {
	switch ravendbv1alpha1.ClusterConditionType(cur.Type) {

	case ravendbv1alpha1.ConditionReady:
		if cur.Status == metav1.ConditionFalse {
			return corev1.EventTypeWarning
		}
		return corev1.EventTypeNormal

	case ravendbv1alpha1.ConditionDegraded:
		if cur.Status == metav1.ConditionTrue {
			return corev1.EventTypeWarning
		}
		return corev1.EventTypeNormal

	case ravendbv1alpha1.ConditionProgressing:
		return corev1.EventTypeNormal

	default:
		if cur.Status == metav1.ConditionFalse {
			return corev1.EventTypeWarning
		}
		return corev1.EventTypeNormal
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RavenDBClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor(common.Manager)
	timing := upgrade.DefaultTiming()
	r.Upgrader = upgrade.NewUpgrader(timing)
	r.BaseTiming = timing

	r.Upgrader.SetEmitter(upgrade.NewGateEventEmitter(r.Client, r.Recorder))

	return ctrl.NewControllerManagedBy(mgr).
		For(&ravendbv1alpha1.RavenDBCluster{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Owns(&batchv1.Job{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
