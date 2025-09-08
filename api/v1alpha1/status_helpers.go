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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *RavenDBCluster) SetConditionTrue(t ClusterConditionType, reason ClusterConditionReason, msg string, now metav1.Time) {
	r.setCondition(t, metav1.ConditionTrue, reason, msg, now)
}

func (r *RavenDBCluster) SetConditionFalse(t ClusterConditionType, reason ClusterConditionReason, msg string, now metav1.Time) {
	r.setCondition(t, metav1.ConditionFalse, reason, msg, now)
}

func (r *RavenDBCluster) SetObservedGeneration(gen int64) {
	r.Status.ObservedGeneration = gen
}

func (r *RavenDBCluster) IsBootstrapped() bool {
	return r.HasConditionTrue(ConditionBootstrapCompleted)
}

func (r *RavenDBCluster) SetBootstrapped(now metav1.Time) {
	r.SetConditionTrue(ConditionBootstrapCompleted, ReasonCompleted, "Bootstrap job succeeded", now)
}

func (r *RavenDBCluster) GetCondition(t ClusterConditionType) (c *metav1.Condition, ok bool) {
	for i := range r.Status.Conditions {
		condition := &r.Status.Conditions[i]

		if condition.Type == string(t) {
			return condition, true
		}
	}
	return nil, false
}

func (r *RavenDBCluster) HasConditionTrue(t ClusterConditionType) bool {
	condition, exists := r.GetCondition(t)
	if !exists {
		return false
	}

	if condition.Status == metav1.ConditionTrue {
		return true
	}

	return false
}

// to ensure we don’t accidentally pass an empty reason
func reasonsanitize(reason ClusterConditionReason) ClusterConditionReason {
	if reason == "" {
		return ReasonCompleted
	}
	return reason
}

func (r *RavenDBCluster) setCondition(t ClusterConditionType, status metav1.ConditionStatus, reason ClusterConditionReason, msg string, now metav1.Time) {

	if c, ok := r.GetCondition(t); ok {

		if c.Status != status {
			c.LastTransitionTime = now
		}

		c.Status = status
		c.Reason = string(reasonsanitize(reason))
		c.Message = msg

		return
	}

	r.Status.Conditions = append(r.Status.Conditions, metav1.Condition{
		Type:               string(t),
		Status:             status,
		Reason:             string(reasonsanitize(reason)),
		Message:            msg,
		LastTransitionTime: now,
	})
}

func (r *RavenDBCluster) ComputeReady(now metav1.Time) {

	required := []ClusterConditionType{
		ConditionCertificatesReady,
		ConditionLicensesValid,
		ConditionStorageReady,
		ConditionNodesHealthy,
		ConditionBootstrapCompleted,
	}

	if r.Spec.ExternalAccessConfiguration != nil {
		required = append(required, ConditionExternalAccessReady)
	}

	for i := 0; i < len(required); i++ {
		conditionType := required[i]
		condition, exists := r.GetCondition(conditionType)

		if !exists {
			r.setCondition(ConditionReady, metav1.ConditionFalse, ClusterConditionReason(string(conditionType)), "", now)
			r.Status.Message = string(conditionType) + " not satisfied"
			return
		}

		if condition.Status != metav1.ConditionTrue {
			r.setCondition(ConditionReady, metav1.ConditionFalse, ClusterConditionReason(string(conditionType)), condition.Message, now)

			if condition.Reason != "" {
				r.Status.Message = condition.Reason + ": " + condition.Message
			} else {
				r.Status.Message = string(conditionType) + " not satisfied"
			}
			return
		}
	}

	r.setCondition(ConditionReady, metav1.ConditionTrue, ReasonCompleted, "Cluster is ready", now)
	r.Status.Message = "Cluster is ready"
}

func (r *RavenDBCluster) UpdatePhaseFromConditions() {
	switch {
	case r.HasConditionTrue(ConditionReady):
		r.Status.Phase = PhaseRunning

	case r.HasConditionTrue(ConditionDegraded):
		r.Status.Phase = PhaseError

	case r.HasConditionTrue(ConditionProgressing):
		r.Status.Phase = PhaseDeploying

	default:
		r.Status.Phase = PhaseDeploying
	}
}
