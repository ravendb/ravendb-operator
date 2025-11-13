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

package upgrade

import (
	"net/http"
	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"strings"
	"time"
)

type GatePhase string

const (
	GatePreStep  GatePhase = "pre-step"
	GatePostStep GatePhase = "post-step"
)

type GateKind string

const (
	GateNodeAlive           GateKind = "node_alive"
	GateClusterConnectivity GateKind = "cluster_connectivity"
	GateDatabasesOnline     GateKind = "db_groups_available_excluding_target"
)

type HealthCheckContext struct {
	http    *http.Client
	baseURL string
	byTag   map[string]string
}

func NewChecks(httpc *http.Client, c *ravendbv1alpha1.RavenDBCluster) *HealthCheckContext {
	leader := ""
	if len(c.Spec.Nodes) > 0 {
		leader = c.Spec.Nodes[0].PublicServerUrl
	}

	urlByTag := map[string]string{}
	for _, n := range c.Spec.Nodes {
		urlByTag[strings.ToUpper(n.Tag)] = n.PublicServerUrl
	}

	if httpc == nil {
		httpc = &http.Client{Timeout: 30 * time.Second}
	}

	return &HealthCheckContext{
		http:    httpc,
		baseURL: strings.TrimRight(leader, "/"),
		byTag:   urlByTag,
	}
}

func (g *HealthCheckContext) urlForTag(tag string) string {
	return g.byTag[strings.ToUpper(tag)]
}
