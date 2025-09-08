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

package health

import (
	"context"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// its purpose to translates K8s objs into ResourceFacts that the evaluator consumes.

func NewResourceCollector() Collector {
	return &resourceCollector{}
}

type resourceCollector struct{}

func (t *resourceCollector) Collect(ctx context.Context, cli client.Client, cluster *ravendbv1alpha1.RavenDBCluster) (*ResourceFacts, error) {

	ns := cluster.Namespace

	facts := &ResourceFacts{
		StatefulSets: make([]StatefulSetFact, 0),
		Pods:         make([]PodFact, 0),
		PVCs:         make([]PVCFact, 0),
		Services:     make([]ServiceFact, 0),
		Ingresses:    make([]IngressFact, 0),
		Jobs:         make([]JobFact, 0),
		Secrets:      make([]SecretFact, 0),
	}

	ssFacts, ownedSSUIDs, err := collectStatefulSets(ctx, cli, ns, cluster)
	if err != nil {
		return facts, err
	}
	facts.StatefulSets = ssFacts

	jobFacts, err := collectJobs(ctx, cli, ns, cluster)
	if err != nil {
		return facts, err
	}
	facts.Jobs = jobFacts

	podFacts, ownedPodUIDs, claimedPVCNames, err := collectPodsAndPVCRefs(ctx, cli, ns, ownedSSUIDs)
	if err != nil {
		return facts, err
	}
	facts.Pods = podFacts

	pvcFacts, err := collectPVCs(ctx, cli, ns, ownedSSUIDs, ownedPodUIDs, claimedPVCNames)
	if err != nil {
		return facts, err
	}
	facts.PVCs = pvcFacts

	svcFacts, err := collectServices(ctx, cli, ns, cluster)
	if err != nil {
		return facts, err
	}
	facts.Services = svcFacts

	ingFacts, err := collectIngresses(ctx, cli, ns, cluster)
	if err != nil {
		return facts, err
	}
	facts.Ingresses = ingFacts

	secFacts, err := collectSecrets(ctx, cli, ns)
	if err != nil {
		return facts, err
	}
	facts.Secrets = secFacts

	return facts, nil
}

func collectStatefulSets(ctx context.Context, cli client.Client, ns string, cluster *ravendbv1alpha1.RavenDBCluster) ([]StatefulSetFact, map[string]struct{}, error) {

	var list appsv1.StatefulSetList
	if err := cli.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return nil, nil, err
	}

	facts := make([]StatefulSetFact, 0, len(list.Items))
	owned := make(map[string]struct{}, len(list.Items))

	for i := 0; i < len(list.Items); i++ {
		ss := &list.Items[i]
		if !isOwnedByCluster(ss.OwnerReferences, cluster) {
			continue
		}

		isUpdating := ss.Status.CurrentRevision != "" &&
			ss.Status.UpdateRevision != "" &&
			ss.Status.CurrentRevision != ss.Status.UpdateRevision

		replicas := int32(1)
		if ss.Spec.Replicas != nil {
			replicas = *ss.Spec.Replicas
		}

		facts = append(facts, StatefulSetFact{
			Name:            ss.Name,
			Namespace:       ss.Namespace,
			Replicas:        replicas,
			ReadyReplicas:   ss.Status.ReadyReplicas,
			CurrentRevision: ss.Status.CurrentRevision,
			UpdateRevision:  ss.Status.UpdateRevision,
			Updating:        isUpdating,
		})

		owned[string(ss.UID)] = struct{}{}
	}

	return facts, owned, nil
}

func collectJobs(ctx context.Context, cli client.Client, ns string, cluster *ravendbv1alpha1.RavenDBCluster) ([]JobFact, error) {

	var list batchv1.JobList
	if err := cli.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return nil, err
	}

	facts := make([]JobFact, 0, len(list.Items))
	for i := 0; i < len(list.Items); i++ {
		j := &list.Items[i]
		if !isOwnedByCluster(j.OwnerReferences, cluster) {
			continue
		}

		jobSucceeded := j.Status.Succeeded >= 1

		facts = append(facts, JobFact{
			Name:      j.Name,
			Namespace: j.Namespace,
			Succeeded: jobSucceeded,
			Active:    j.Status.Active,
			Failed:    j.Status.Failed,
			Completed: jobSucceeded,
		})
	}

	return facts, nil
}

func collectPodsAndPVCRefs(ctx context.Context, cli client.Client, ns string, ownedSSUIDs map[string]struct{}) ([]PodFact, map[string]struct{}, map[string]struct{}, error) {

	var list corev1.PodList
	if err := cli.List(ctx, &list, client.InNamespace(ns)); err != nil {
		return nil, nil, nil, err
	}

	podFacts := make([]PodFact, 0, len(list.Items))
	ownedPodUIDs := make(map[string]struct{}, len(list.Items))
	claimedPVCNames := make(map[string]struct{})

	for i := 0; i < len(list.Items); i++ {
		p := &list.Items[i]
		if !isOwnedByAny(p.OwnerReferences, ownedSSUIDs) {
			continue
		}

		ownedPodUIDs[string(p.UID)] = struct{}{}
		podFacts = append(podFacts, PodFact{
			Name:      p.Name,
			Namespace: p.Namespace,
			Phase:     string(p.Status.Phase),
			Ready:     isPodReady(p),
			Restarts:  getPodsContainersTotalRestarts(p),
		})

		for _, vol := range p.Spec.Volumes {
			pvcRef := vol.PersistentVolumeClaim

			if pvcRef == nil || pvcRef.ClaimName == "" {
				continue
			}
			claimedPVCNames[pvcRef.ClaimName] = struct{}{}
		}
	}

	return podFacts, ownedPodUIDs, claimedPVCNames, nil
}

func collectPVCs(
	ctx context.Context,
	cli client.Client,
	ns string,
	ownedSSUIDs map[string]struct{},
	ownedPodUIDs map[string]struct{},
	claimedPVCNames map[string]struct{},
) ([]PVCFact, error) {

	var pvcList corev1.PersistentVolumeClaimList
	if err := cli.List(ctx, &pvcList, client.InNamespace(ns)); err != nil {
		return nil, err
	}

	facts := make([]PVCFact, 0, len(pvcList.Items))

	// a pvc belongs to us if: it’s owned by one of our STSs/Pods or one of our Pods referenced it by claim name (so also any additional vols)
	for _, pvc := range pvcList.Items {

		ownedBySS := isOwnedByAny(pvc.OwnerReferences, ownedSSUIDs)
		ownedByPod := isOwnedByAny(pvc.OwnerReferences, ownedPodUIDs)
		_, referencedByPod := claimedPVCNames[pvc.Name]

		if !(ownedBySS || ownedByPod || referencedByPod) {
			continue
		}

		requested := ""
		actual := ""
		if q, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			requested = q.String()
		}
		if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			actual = q.String()
		}

		facts = append(facts, PVCFact{
			Name:          pvc.Name,
			Namespace:     pvc.Namespace,
			Bound:         pvc.Status.Phase == corev1.ClaimBound,
			Phase:         string(pvc.Status.Phase),
			RequestedSize: requested,
			ActualSize:    actual,
		})
	}

	return facts, nil
}

func collectServices(ctx context.Context, cli client.Client, ns string, cluster *ravendbv1alpha1.RavenDBCluster) ([]ServiceFact, error) {

	var svcList corev1.ServiceList
	if err := cli.List(ctx, &svcList, client.InNamespace(ns)); err != nil {
		return nil, err
	}

	facts := make([]ServiceFact, 0, len(svcList.Items))
	for i := 0; i < len(svcList.Items); i++ {
		svc := &svcList.Items[i]
		if !isOwnedByCluster(svc.OwnerReferences, cluster) {
			continue
		}

		lbReady := len(svc.Status.LoadBalancer.Ingress) > 0
		hasClusterIP := svc.Spec.ClusterIP != "" && svc.Spec.ClusterIP != corev1.ClusterIPNone

		facts = append(facts, ServiceFact{
			Name:         svc.Name,
			Namespace:    svc.Namespace,
			Type:         string(svc.Spec.Type),
			HasClusterIP: hasClusterIP,
			LBReady:      lbReady,
		})
	}

	return facts, nil
}

func collectIngresses(ctx context.Context, cli client.Client, ns string, cluster *ravendbv1alpha1.RavenDBCluster) ([]IngressFact, error) {

	var ingList networkingv1.IngressList
	if err := cli.List(ctx, &ingList, client.InNamespace(ns)); err != nil {
		return nil, err
	}

	facts := make([]IngressFact, 0, len(ingList.Items))
	for _, ing := range ingList.Items {
		if !isOwnedByCluster(ing.OwnerReferences, cluster) {
			continue
		}
		hasLBIngress := len(ing.Status.LoadBalancer.Ingress) > 0

		facts = append(facts, IngressFact{
			Name:      ing.Name,
			Namespace: ing.Namespace,
			LBReady:   hasLBIngress,
		})
	}

	return facts, nil
}

func collectSecrets(ctx context.Context, cli client.Client, ns string) ([]SecretFact, error) {

	var secretList corev1.SecretList
	if err := cli.List(ctx, &secretList, client.InNamespace(ns)); err != nil {
		return nil, err
	}

	facts := make([]SecretFact, 0, len(secretList.Items))

	for _, s := range secretList.Items {
		facts = append(facts, SecretFact{
			Name:      s.Name,
			Namespace: s.Namespace,
			Type:      string(s.Type),
		})
	}
	return facts, nil
}

func isOwnedByCluster(owners []metav1.OwnerReference, cluster *ravendbv1alpha1.RavenDBCluster) bool {
	for i := 0; i < len(owners); i++ {
		owner := owners[i]
		if owner.Kind == "RavenDBCluster" && owner.UID == cluster.UID {
			return true
		}
	}
	return false
}

func isOwnedByAny(owners []metav1.OwnerReference, allowedUIDs map[string]struct{}) bool {
	for i := 0; i < len(owners); i++ {
		if _, ok := allowedUIDs[string(owners[i].UID)]; ok {
			return true
		}
	}
	return false
}

func isPodReady(p *corev1.Pod) bool {
	for i := 0; i < len(p.Status.Conditions); i++ {
		c := p.Status.Conditions[i]
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func getPodsContainersTotalRestarts(p *corev1.Pod) int32 {
	var sum int32

	for i := 0; i < len(p.Status.ContainerStatuses); i++ {
		sum += p.Status.ContainerStatuses[i].RestartCount
	}
	return sum
}
