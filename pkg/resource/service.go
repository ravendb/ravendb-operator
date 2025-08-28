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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceBuilder struct{}

func NewServiceBuilder() PerNodeBuilder {
	return &ServiceBuilder{}
}

func (b *ServiceBuilder) Build(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) (client.Object, error) {
	return BuildService(cluster, node)
}

func BuildService(cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) (*corev1.Service, error) {

	svcName := fmt.Sprintf("%s%s", common.Prefix, node.Tag)

	labels := buildServiceLabels(cluster, node)
	ports := buildServicePorts()
	selector := buildServiceSelector(node)

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: cluster.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports:    ports,
		},
	}

	access := cluster.Spec.ExternalAccessConfiguration
	if access != nil {
		modifyServiceForExternalAccess(svc, access, node.Tag)
	}

	return svc, nil
}

func buildServiceLabels(cluster *ravendbv1alpha1.RavenDBCluster, node ravendbv1alpha1.RavenDBNode) map[string]string {
	return map[string]string{
		common.LabelAppName:   common.App,
		common.LabelManagedBy: common.Manager,
		common.LabelInstance:  cluster.Name,
		common.LabelNodeTag:   node.Tag,
	}
}

func buildServiceSelector(node ravendbv1alpha1.RavenDBNode) map[string]string {
	return map[string]string{
		common.LabelNodeTag: node.Tag,
	}
}

func buildServicePorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Name: common.HttpsPortName,
			Port: common.InternalHttpsPort,
		},
		{
			Name:     common.TcpPortName,
			Port:     common.InternalTcpPort,
			Protocol: corev1.ProtocolTCP,
		},
	}
}
func modifyServiceForExternalAccess(svc *corev1.Service, access *ravendbv1alpha1.ExternalAccessConfiguration, tag string) {

	switch access.Type {

	case ravendbv1alpha1.ExternalAccessTypeAWS:
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster
		if svc.Annotations == nil {
			svc.Annotations = map[string]string{}
		}
		for k, v := range buildAwsNlbAnnotations(access.AWSExternalAccess, tag) {
			svc.Annotations[k] = v
		}

	case ravendbv1alpha1.ExternalAccessTypeAzure:
		svc.Spec.Type = corev1.ServiceTypeLoadBalancer
		svc.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster

		if access.AzureExternalAccess != nil {
			for _, mapping := range access.AzureExternalAccess.NodeMappings {
				if mapping.Tag == tag {
					svc.Spec.LoadBalancerIP = mapping.IP
					break
				}
			}
		}

	case ravendbv1alpha1.ExternalAccessTypeIngressController:
		// do nothing
	}

}

func buildAwsNlbAnnotations(cfg *ravendbv1alpha1.AWSExternalAccessContext, tag string) map[string]string {
	for _, m := range cfg.NodeMappings {
		if m.Tag == tag {
			return map[string]string{
				common.AWSLoadBalancerTypeAnnotation:           "external",
				common.AWSLoadBalancerNLBTargetTypeAnnotation:  "ip",
				common.AWSLoadBalancerSchemeAnnotation:         "internet-facing",
				common.AWSLoadBalancerEIPAllocationsAnnotation: m.EIPAllocationId,
				common.AWSLoadBalancerSubnetsAnnotation:        m.SubnetId,
			}
		}
	}
	return map[string]string{}
}
