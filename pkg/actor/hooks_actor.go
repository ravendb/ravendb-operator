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

package actor

import (
	"context"
	"fmt"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/assets"
	"ravendb-operator/pkg/common"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type HooksActor struct{}

func NewHooksActor() PerClusterActor {
	return &HooksActor{}
}

func (a *HooksActor) Name() string {
	return "HooksActor"
}

func (a *HooksActor) ShouldAct(cluster *ravendbv1.RavenDBCluster) bool {
	return true
}

func (a *HooksActor) Act(
	ctx context.Context,
	cluster *ravendbv1.RavenDBCluster,
	c client.Client,
	scheme *runtime.Scheme,
) (bool, error) {

	bootstrapperCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.BootstrapperHookConfigMap,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				common.LabelAppName:   common.App,
				common.LabelManagedBy: common.Manager,
				common.LabelInstance:  cluster.Name,
			},
		},
		Data: map[string]string{
			common.InitClusterHookKey:               assets.InitClusterScript,
			common.CheckNodesDiscoverabilityHookKey: assets.CheckNodesDiscoverabilityScript,
		},
	}

	if err := controllerutil.SetControllerReference(cluster, bootstrapperCM, scheme); err != nil {
		return false, fmt.Errorf("set owner ref on bootstrapper hook ConfigMap: %w", err)
	}

	if _, err := applyResourceSSA(ctx, c, bootstrapperCM, "ravendb-operator/hooks"); err != nil {
		return false, fmt.Errorf("apply bootstrapper hook ConfigMap: %w", err)
	}

	certHookCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.CertHookConfigMap,
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				common.LabelAppName:   common.App,
				common.LabelManagedBy: common.Manager,
				common.LabelInstance:  cluster.Name,
			},
		},
		Data: map[string]string{
			common.UpdateCertHookKey: assets.UpdateCertScript,
			common.GetCertHookKey:    assets.GetServerCertScript,
		},
	}

	if err := controllerutil.SetControllerReference(cluster, certHookCM, scheme); err != nil {
		return false, fmt.Errorf("set owner ref on cert hook ConfigMap: %w", err)
	}

	if _, err := applyResourceSSA(ctx, c, certHookCM, "ravendb-operator/hooks"); err != nil {
		return false, fmt.Errorf("apply cert hook ConfigMap: %w", err)
	}

	return false, nil
}
