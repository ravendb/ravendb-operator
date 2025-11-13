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
	"context"
	"fmt"
	"strings"
)

func (hcc *HealthCheckContext) NodeAlive(ctx context.Context, tag string) (bool, string, error) {
	nodeURL := strings.TrimSpace(hcc.urlForTag(tag))
	if nodeURL == "" {
		return false, "empty node url", fmt.Errorf("no URL for tag %q", tag)
	}

	endpoint, err := join(nodeURL, "/setup/alive")
	if err != nil {
		return false, err.Error(), nil
	}

	code, body, err := hcc.httpGET(ctx, endpoint)
	if err != nil {
		return false, err.Error(), nil
	}

	if code >= 200 && code < 300 {
		return true, fmt.Sprintf("status:%d", code), nil
	}

	return false, fmt.Sprintf("HTTP %d (%s)", code, truncate(body, 200)), nil
}
