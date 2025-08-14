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

// todo: this file will be rewritten and heavily used in the health probe issues

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func (r *RavenDBCluster) IsBootstrapped() bool {
	for i := range r.Status.Conditions {
		c := r.Status.Conditions[i]
		if c.Type == ConditionBootstrapped && c.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

func (r *RavenDBCluster) SetBootstrapped(now metav1.Time) {
	newC := metav1.Condition{
		Type:               ConditionBootstrapped,
		Status:             metav1.ConditionTrue,
		Reason:             ReasonBootstrapCompleted,
		Message:            "Bootstrap job succeeded",
		LastTransitionTime: now,
	}
	for i := range r.Status.Conditions {
		if r.Status.Conditions[i].Type == ConditionBootstrapped {
			r.Status.Conditions[i] = newC
			return
		}
	}
	r.Status.Conditions = append(r.Status.Conditions, newC)
}
