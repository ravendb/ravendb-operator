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
	"ravendb-operator/pkg/webhook/adapter"

	"k8s.io/apimachinery/pkg/util/errors"
)

type ClusterAdapter = adapter.ClusterAdapter

type Mutator interface {
	Name() string
	Mutate(cluster ClusterAdapter) MutationResult
}

type MutationResult struct {
	Err     error
	Warning string
}

var mutators []Mutator

func Register(m Mutator) {
	mutators = append(mutators, m)
}

func Run(cluster ClusterAdapter) (warnings []string, err error) {
	var errs []error

	for _, m := range mutators {
		res := m.Mutate(cluster)
		if res.Err != nil {
			errs = append(errs, fmt.Errorf("[%s] %w", m.Name(), res.Err))
		}
		if res.Warning != "" {
			warnings = append(warnings, fmt.Sprintf("[%s] %s", m.Name(), res.Warning))
		}

	}

	return warnings, errors.NewAggregate(errs)
}
