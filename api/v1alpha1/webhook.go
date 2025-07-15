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
	"context"
	"fmt"

	"ravendb-operator/pkg/webhook"
	"ravendb-operator/pkg/webhook/validator"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	sigswebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var ravendbclusterlog = logf.Log.WithName("ravendbcluster-resource")

func (r *RavenDBCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	validator.Register(validator.NewImageValidator(mgr.GetClient()))
	validator.Register(validator.NewGeneralValidator(mgr.GetClient()))
	validator.Register(validator.NewNodeValidator(mgr.GetClient()))
	validator.Register(validator.NewEaValidator(mgr.GetClient()))
	validator.Register(validator.NewStorageValidator(mgr.GetClient()))

	return ctrl.NewWebhookManagedBy(mgr).For(r).Complete()
}

// +kubebuilder:webhook:path=/validate-ravendb-ravendb-io-v1alpha1-ravendbcluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=ravendb.ravendb.io,resources=ravendbclusters,verbs=create;update,versions=v1alpha1,name=vravendbcluster.kb.io,admissionReviewVersions=v1

var _ sigswebhook.Validator = &RavenDBCluster{}

func (r *RavenDBCluster) ValidateCreate() (admission.Warnings, error) {
	ravendbclusterlog.Info("validate create", "name", r.Name)
	return nil, webhook.ValidateCreate(context.TODO(), r)
}

func (r *RavenDBCluster) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldCluster, ok := old.(*RavenDBCluster)
	if !ok {
		return nil, fmt.Errorf("expected *RavenDBCluster but got %T", old)
	}
	ravendbclusterlog.Info("validate update", "name", r.Name)
	return nil, webhook.ValidateUpdate(context.TODO(), oldCluster, r)
}

func (r *RavenDBCluster) ValidateDelete() (admission.Warnings, error) {
	ravendbclusterlog.Info("validate delete", "name", r.Name)
	return nil, nil
}

// +kubebuilder:webhook:path=/mutate-ravendb-ravendb-io-v1alpha1-ravendbcluster,mutating=true,failurePolicy=fail,sideEffects=None,groups=ravendb.ravendb.io,resources=ravendbclusters,verbs=create;update,versions=v1alpha1,name=mravendbcluster.kb.io,admissionReviewVersions=v1

var _ sigswebhook.Defaulter = &RavenDBCluster{}

func (r *RavenDBCluster) Default() {
	ravendbclusterlog.Info("mutate default", "name", r.Name)
	if err := webhook.Default(r); err != nil {
		ravendbclusterlog.Error(err, "mutation failed")
	}
}
