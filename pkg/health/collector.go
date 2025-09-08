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

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Collector interface {
	Collect(ctx context.Context, c client.Client, cluster *ravendbv1alpha1.RavenDBCluster) (*ResourceFacts, error)
}

type ResourceFacts struct {
	StatefulSets []StatefulSetFact
	Pods         []PodFact
	PVCs         []PVCFact
	Services     []ServiceFact
	Ingresses    []IngressFact
	Jobs         []JobFact
	Secrets      []SecretFact
}

type StatefulSetFact struct {
	Name            string
	Namespace       string
	Replicas        int32
	ReadyReplicas   int32
	CurrentRevision string
	UpdateRevision  string
	Updating        bool
}

type PodFact struct {
	Name      string
	Namespace string
	Phase     string
	Ready     bool
	Restarts  int32
}

type PVCFact struct {
	Name          string
	Namespace     string
	Bound         bool
	Phase         string
	RequestedSize string
	ActualSize    string
}

type ServiceFact struct {
	Name         string
	Namespace    string
	Type         string
	HasClusterIP bool
	LBReady      bool
}

type IngressFact struct {
	Name      string
	Namespace string
	LBReady   bool
}

type JobFact struct {
	Name      string
	Namespace string
	Succeeded bool
	Active    int32
	Failed    int32
	Completed bool
}

type SecretFact struct {
	Name      string
	Namespace string
	Type      string
}
