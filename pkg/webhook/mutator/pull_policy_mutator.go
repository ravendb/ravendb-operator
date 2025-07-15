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

package mutator

import (
	"fmt"
	"strings"
)

type pullPolicyMutator struct{}

func (m *pullPolicyMutator) Name() string {
	return "pull-policy-mutator"
}

func (m *pullPolicyMutator) Mutate(c ClusterAdapter) MutationResult {
	image := c.GetImage()
	if strings.HasSuffix(image, ":latest") {
		c.SetIpp("Always")
		return MutationResult{
			Warning: fmt.Sprintf("image %q uses ':latest' - setting imagePullPolicy to 'Always'", image),
		}
	}
	return MutationResult{}
}

func init() {
	Register(&pullPolicyMutator{})
}
