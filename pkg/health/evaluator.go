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
	"sort"
	"strings"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Evaluator interface {
	Evaluate(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts, now metav1.Time)
}

type evaluator struct{}

type conditionResult struct {
	status  metav1.ConditionStatus
	reason  ravendbv1alpha1.ClusterConditionReason
	message string
	skip    bool
}

func NewEvaluator() Evaluator {
	return &evaluator{}
}

func (e *evaluator) Evaluate(_ context.Context, cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts, now metav1.Time) {

	e.apply(cluster, ravendbv1alpha1.ConditionStorageReady, e.evalStorage(cluster, res), now)
	e.apply(cluster, ravendbv1alpha1.ConditionCertificatesReady, e.evalCertificates(cluster, res), now)
	e.apply(cluster, ravendbv1alpha1.ConditionLicensesValid, e.evalLicense(cluster, res), now)
	e.apply(cluster, ravendbv1alpha1.ConditionNodesHealthy, e.evalNodesHealthy(cluster, res), now)
	e.apply(cluster, ravendbv1alpha1.ConditionExternalAccessReady, e.evalExternalAccessReady(cluster, res), now)
	e.apply(cluster, ravendbv1alpha1.ConditionBootstrapCompleted, e.evalBootstrap(cluster, res), now)
	e.apply(cluster, ravendbv1alpha1.ConditionProgressing, e.evalProgressingCase(cluster, res), now)
	e.apply(cluster, ravendbv1alpha1.ConditionDegraded, e.evalDegradingCase(cluster, res), now)

	cluster.SetObservedGeneration(cluster.Generation)
	cluster.ComputeReady(now)
	cluster.UpdatePhaseFromConditions()
}

func (e *evaluator) apply(cluster *ravendbv1alpha1.RavenDBCluster, condType ravendbv1alpha1.ClusterConditionType, r conditionResult, now metav1.Time) {

	if r.skip {
		return
	}

	if r.status == metav1.ConditionTrue {
		cluster.SetConditionTrue(condType, r.reason, r.message, now)
		return
	}

	cluster.SetConditionFalse(condType, r.reason, r.message, now)
}

func (e *evaluator) evalStorage(cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts) conditionResult {

	if res == nil || len(res.PVCs) == 0 {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonPVCNotBound, message: "waiting for PVCs to be created/bound"}
	}

	notBound := make([]string, 0, len(res.PVCs))

	for i := 0; i < len(res.PVCs); i++ {
		p := res.PVCs[i]
		if !p.Bound {
			notBound = append(notBound, p.Namespace+"/"+p.Name)
			continue
		}
	}

	if len(notBound) > 0 {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonPVCNotBound, message: "PVCs not bound: " + joinNames(notBound)}
	}

	return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonCompleted, message: "all PVCs bound"}
}

func (e *evaluator) evalBootstrap(cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts) conditionResult {

	if res == nil || len(res.Jobs) == 0 {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonBootstrapJobRunning, message: "bootstrap job not observed yet"}
	}

	for i := 0; i < len(res.Jobs); i++ {
		if res.Jobs[i].Completed {
			return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonCompleted, message: "bootstrap job succeeded"}
		}
	}

	for i := 0; i < len(res.Jobs); i++ {
		if res.Jobs[i].Failed > 0 {
			return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonBootstrapFailed, message: "bootstrap job failed"}
		}
	}

	return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonBootstrapJobRunning, message: "bootstrap job still running"}
}

func (e *evaluator) evalCertificates(cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts) conditionResult {

	if res == nil {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonCertSecretMissing, message: "waiting for certificate secrets to be observed"}
	}

	expectedSecrets := getExpectedSecretNames(cluster)
	if len(expectedSecrets) == 0 {
		return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonCompleted, message: "no certificate secrets required"}
	}

	observedSecrets := getSecretNamesSet(res.Secrets)
	missingSecrets := getMissingSecrets(cluster.Namespace, expectedSecrets, observedSecrets)

	if len(missingSecrets) > 0 {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonCertSecretMissing, message: "missing certificate secrets: " + joinNames(missingSecrets)}
	}

	return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonCompleted, message: "all certificate secrets present"}
}

func (e *evaluator) evalNodesHealthy(cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts) conditionResult {

	if res == nil || len(res.Pods) == 0 {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonWaitingForPods, message: "waiting for pods to be created/scheduled"}
	}

	pending, failed, unknown, notReady := bucketizePodsByState(res.Pods)

	if len(pending) > 0 {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonWaitingForPods, message: "pods pending: " + joinNames(pending)}
	}

	if len(failed) > 0 || len(unknown) > 0 {
		msg := ""
		if len(failed) > 0 {
			msg += "pods failed: " + joinNames(failed)
		}
		if len(unknown) > 0 {
			if msg != "" {
				msg += "; "
			}
			msg += "pods unknown: " + joinNames(unknown)
		}

		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonPodsNotReady, message: msg}
	}

	if len(notReady) > 0 {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonPodsNotReady, message: "pods not ready: " + joinNames(notReady)}
	}

	return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonCompleted, message: "all node pods ready"}
}

func (e *evaluator) evalLicense(cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts) conditionResult {

	license := cluster.Spec.LicenseSecretRef
	if license == "" {
		return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonCompleted, message: "no license ref in spec"}
	}

	if res == nil {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonLicenseSecretMissing, message: "waiting for secrets to be observed"}
	}

	for i := 0; i < len(res.Secrets); i++ {
		if res.Secrets[i].Name == license {
			return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonCompleted, message: "license secret present"}
		}
	}

	return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonLicenseSecretMissing, message: "missing license secret: " + cluster.Namespace + "/" + license}
}

func (e *evaluator) evalExternalAccessReady(cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts) conditionResult {

	// only when external access is configured.
	if cluster.Spec.ExternalAccessConfiguration == nil {
		return conditionResult{skip: true}
	}

	if res == nil {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonLoadBalancerPending, message: "waiting for ingress/load balancer to be observed"}
	}

	ingressObserved, ingressReady := getIngressesStatus(res.Ingresses)
	if ingressReady {
		return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonCompleted, message: "ingress load balancer address allocated"}
	}
	if ingressObserved {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonIngressPendingAddress, message: "waiting for ingress load balancer address"}
	}

	svcObserved, svcReady := getLbServicesStatus(res.Services)
	if svcReady {
		return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonCompleted, message: "service load balancer address allocated"}
	}
	if svcObserved {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonLoadBalancerPending, message: "waiting for service load balancer address"}
	}

	return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonLoadBalancerPending, message: "no ingress/load balancer service observed"}
}

// Progressing=True when any of the STSs is updating or bootstrap job is active.
func (e *evaluator) evalProgressingCase(cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts) conditionResult {
	if res == nil {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonCompleted, message: "no active rollouts"}
	}

	for i := 0; i < len(res.StatefulSets); i++ {
		if res.StatefulSets[i].Updating {
			return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonStatefulSetUpdating, message: "rollout in progress"}
		}
	}

	for i := 0; i < len(res.Jobs); i++ {
		if res.Jobs[i].Active > 0 {
			return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonStatefulSetUpdating, message: "rollout in progress"}
		}
	}

	return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonCompleted, message: "no active rollouts"}
}

// Degraded=True when bootstrap job failed or pods have high restart counts.
func (e *evaluator) evalDegradingCase(cluster *ravendbv1alpha1.RavenDBCluster, res *ResourceFacts) conditionResult {

	if c, ok := cluster.GetCondition(ravendbv1alpha1.ConditionBootstrapCompleted); ok &&
		c.Status == metav1.ConditionFalse && c.Reason == string(ravendbv1alpha1.ReasonBootstrapFailed) {
		return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonBootstrapFailed, message: "bootstrap job failed"}
	}

	if res == nil {
		return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonCompleted, message: "no degradation detected"}
	}

	const restartThreshold int32 = 5
	offendersPods := make([]string, 0, len(res.Pods))

	for i := 0; i < len(res.Pods); i++ {
		pod := res.Pods[i]
		if pod.Restarts >= restartThreshold {
			offendersPods = append(offendersPods, pod.Namespace+"/"+pod.Name)
		}
	}

	if len(offendersPods) > 0 {
		return conditionResult{status: metav1.ConditionTrue, reason: ravendbv1alpha1.ReasonPodsNotReady, message: "high restart count: " + joinNames(offendersPods)}
	}

	return conditionResult{status: metav1.ConditionFalse, reason: ravendbv1alpha1.ReasonCompleted, message: "no degradation detected"}
}

func getExpectedSecretNames(cluster *ravendbv1alpha1.RavenDBCluster) []string {
	secretsList := []string{}

	switch cluster.Spec.Mode {
	case ravendbv1alpha1.ModeLetsEncrypt:

		if cluster.Spec.ClientCertSecretRef != "" {
			secretsList = append(secretsList, cluster.Spec.ClientCertSecretRef)
		}

		for i := range cluster.Spec.Nodes {
			if cluster.Spec.Nodes[i].CertSecretRef != nil {
				secretsList = append(secretsList, *cluster.Spec.Nodes[i].CertSecretRef)
			}
		}

	case ravendbv1alpha1.ModeNone:

		if cluster.Spec.ClientCertSecretRef != "" {
			secretsList = append(secretsList, cluster.Spec.ClientCertSecretRef)
		}

		if cluster.Spec.ClusterCertSecretRef != nil {
			secretsList = append(secretsList, *cluster.Spec.ClusterCertSecretRef)
		}

		if cluster.Spec.CACertSecretRef != nil {
			secretsList = append(secretsList, *cluster.Spec.CACertSecretRef)
		}
	}

	return secretsList
}

func getSecretNamesSet(secrets []SecretFact) map[string]struct{} {
	set := make(map[string]struct{}, len(secrets))

	for i := 0; i < len(secrets); i++ {
		set[secrets[i].Name] = struct{}{}
	}
	return set
}

func getMissingSecrets(namespace string, expected []string, observedSecrets map[string]struct{}) []string {
	secretsArr := make([]string, 0, len(expected))

	for i := 0; i < len(expected); i++ {
		name := expected[i]
		if _, ok := observedSecrets[name]; !ok {
			secretsArr = append(secretsArr, namespace+"/"+name)
		}
	}

	if len(secretsArr) > 1 {
		sort.Strings(secretsArr)
	}

	return secretsArr
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.Join(names, ", ")
}

func bucketizePodsByState(pods []PodFact) (pending, failed, unknown, notReady []string) {
	const (
		phasePending = string(corev1.PodPending)
		phaseFailed  = string(corev1.PodFailed)
		phaseUnknown = string(corev1.PodUnknown)
		phaseRunning = string(corev1.PodRunning)
	)
	for i := 0; i < len(pods); i++ {
		pod := pods[i]
		name := pod.Namespace + "/" + pod.Name

		switch pod.Phase {
		case phasePending:
			pending = append(pending, name)

		case phaseFailed:
			failed = append(failed, name)

		case phaseUnknown:
			unknown = append(unknown, name)

		case phaseRunning:
			if !pod.Ready {
				notReady = append(notReady, name)
			}
		}
	}
	return
}

func getIngressesStatus(ing []IngressFact) (observed bool, ready bool) {

	for i := 0; i < len(ing); i++ {
		observed = true
		if ing[i].LBReady {
			ready = true
			return
		}
	}
	return
}

func getLbServicesStatus(svcs []ServiceFact) (observed bool, ready bool) {
	lbType := string(corev1.ServiceTypeLoadBalancer)

	for i := 0; i < len(svcs); i++ {
		if svcs[i].Type != lbType {
			continue
		}

		observed = true
		if svcs[i].LBReady {
			ready = true
			return
		}
	}
	return
}
