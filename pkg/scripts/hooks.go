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

package scripts

import (
	_ "embed"
	"strings"
)

//go:embed check-nodes-discoverability.sh
var checkNodesDiscoverabilityScriptRaw string

//go:embed init-cluster.sh
var initClusterScriptRaw string

//go:embed update-cert.sh
var updateCertScriptRaw string

//go:embed get-server-cert.sh
var getServerCertScriptRaw string

func normalizeLF(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

var (
	CheckNodesDiscoverabilityScript = normalizeLF(checkNodesDiscoverabilityScriptRaw)
	InitClusterScript               = normalizeLF(initClusterScriptRaw)
	UpdateCertScript                = normalizeLF(updateCertScriptRaw)
	GetServerCertScript             = normalizeLF(getServerCertScriptRaw)
)
