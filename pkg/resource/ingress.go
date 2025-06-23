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

package resource

import (
	"context"
	"fmt"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"ravendb-operator/pkg/common"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	client "sigs.k8s.io/controller-runtime/pkg/client"
)

type IngressBuilder struct{}

func NewIngressBuilder() PerClusterBuilder {
	return &IngressBuilder{}
}

func (b *IngressBuilder) Build(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster) (client.Object, error) {
	return BuildIngress(cluster)
}

func BuildIngress(cluster *ravendbv1alpha1.RavenDBCluster) (*networkingv1.Ingress, error) {
	ingressName := common.App

	labels := buildIngressLabels(cluster)
	annotations := buildIngressAnnotations(cluster)
	rules := buildIngressRules(cluster)

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   cluster.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &cluster.Spec.ExternalAccessConfiguration.IngressControllerExternalAccess.IngressClassName,
			Rules:            rules,
		},
	}

	return ing, nil
}

func buildIngressLabels(cluster *ravendbv1alpha1.RavenDBCluster) map[string]string {
	return map[string]string{
		common.LabelAppName:   common.App,
		common.LabelManagedBy: common.Manager,
		common.LabelInstance:  cluster.Name,
	}
}

func buildIngressAnnotations(cluster *ravendbv1alpha1.RavenDBCluster) map[string]string {
	annotations := map[string]string{
		common.IngressSSLPassthroughAnnotation: "true",
	}

	ic := cluster.Spec.ExternalAccessConfiguration.IngressControllerExternalAccess
	////////////////////////////////////////////////////////////////////////////
	// to be removed - validation and fallback should be done in webhooks
	if ic == nil {
		return annotations
	}
	////////////////////////////////////////////////////////////////////////////

	switch ic.IngressClassName {

	case "nginx":
		annotations[common.NginxSSLPassthroughAnnotation] = "true"
	case "haproxy":
		// placehodler , TODO
	case "traefik":
		// placehodler , TODO
		// Note: traefik has it's own philosophy of wiring up their ingress controller https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/crd/tcp/ingressroutetcp/
	}

	for k, v := range ic.AdditionalAnnotations {
		annotations[k] = v
	}

	return annotations
}

func buildIngressRules(cluster *ravendbv1alpha1.RavenDBCluster) []networkingv1.IngressRule {
	var rules []networkingv1.IngressRule

	for _, node := range cluster.Spec.Nodes {
		rules = append(rules,
			buildHTTPSRule(node.Tag, cluster.Spec.Domain),
			buildTCPRule(node.Tag, cluster.Spec.Domain),
		)
	}

	return rules
}

func buildHTTPSRule(nodeName, domain string) networkingv1.IngressRule {
	return networkingv1.IngressRule{
		Host: fmt.Sprintf("%s.%s", nodeName, domain),
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path:     "/",
						PathType: pathTypePtr(networkingv1.PathTypePrefix),
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: fmt.Sprintf("%s%s", common.Prefix, nodeName),
								Port: networkingv1.ServiceBackendPort{Number: common.InternalHttpsPort},
							},
						},
					},
				},
			},
		},
	}
}

func buildTCPRule(nodeName, domain string) networkingv1.IngressRule {
	return networkingv1.IngressRule{
		Host: fmt.Sprintf("%s-tcp.%s", nodeName, domain),
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path:     "/",
						PathType: pathTypePtr(networkingv1.PathTypePrefix),
						Backend: networkingv1.IngressBackend{
							Service: &networkingv1.IngressServiceBackend{
								Name: fmt.Sprintf("%s%s", common.Prefix, nodeName),
								Port: networkingv1.ServiceBackendPort{Number: common.InternalTcpPort},
							},
						},
					},
				},
			},
		},
	}
}

func pathTypePtr(pt networkingv1.PathType) *networkingv1.PathType {
	return &pt
}
