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

package validator

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type imageValidator struct {
	client client.Reader
}

func NewImageValidator(c client.Reader) *imageValidator {
	return &imageValidator{client: c}
}

func (v *imageValidator) Name() string {
	return "image-validator"
}

func (v *imageValidator) ValidateCreate(_ context.Context, c ClusterAdapter) error {
	image := c.GetImage()

	if !strings.HasPrefix(image, "ravendb/") {
		return fmt.Errorf("image must be from the 'ravendb/' registry namespace")
	}

	// todo note:  wanted to enforce tag format, but it is not possible to do it in a generic way (ravendb images are not standardized)
	// todo note:  wanted to reject images with version < 5.2.x but it is not possible to do it in a generic way (ravendb images are not standardized)
	return nil
}

func (v *imageValidator) ValidateUpdate(ctx context.Context, _, newC ClusterAdapter) error {
	return v.ValidateCreate(ctx, newC)
}

func init() {
	Register(&imageValidator{})
}
