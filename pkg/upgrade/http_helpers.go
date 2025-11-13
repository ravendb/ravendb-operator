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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type pingItem struct {
	Url        string
	SetupAlive struct{ Error string }
	TcpInfo    struct{ Error string }
}

type pingResponse struct {
	Result []pingItem
}

func (hcc *HealthCheckContext) httpGET(ctx context.Context, rawURL string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, "", err
	}
	resp, err := hcc.http.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		return resp.StatusCode, "", rerr
	}
	return resp.StatusCode, string(body), nil
}

func (hcc *HealthCheckContext) clusterURL() (string, error) {
	if hcc.baseURL == "" {
		return "", errors.New("baseURL is empty")
	}
	return hcc.baseURL, nil
}

func collapseWhitespace(s string) string { return strings.Join(strings.Fields(s), " ") }

func summarizeError(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " ---"); i > 0 { // drop inner exceptions
		s = s[:i]
	}
	s = collapseWhitespace(s)

	const max = 160
	r := []rune(s)
	if len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

func join(base, path string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("empty base url")
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/"), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
