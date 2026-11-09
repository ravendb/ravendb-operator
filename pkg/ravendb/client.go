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

package ravendb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ravendbv1 "ravendb-operator/api/v1"
)

const requestTimeout = 30 * time.Second

// Client calls the admin REST API of a RavenDB cluster. Each request goes to the first node that answers.
type Client struct {
	http *http.Client
	urls []string
}

func NewClient(httpc *http.Client, urls ...string) *Client {
	return &Client{http: httpc, urls: urls}
}

// ForCluster connects to the cluster's public node URLs with its admin client certificate.
func ForCluster(ctx context.Context, kc client.Client, c *ravendbv1.RavenDBCluster) (*Client, error) {
	httpc, err := NewHTTPClient(ctx, kc, c)
	if err != nil {
		return nil, err
	}
	httpc.Timeout = requestTimeout

	urls := make([]string, 0, len(c.Spec.Nodes))
	for _, n := range c.Spec.Nodes {
		urls = append(urls, strings.TrimRight(n.PublicServerUrl, "/"))
	}
	return NewClient(httpc, urls...), nil
}

// APIError is a failure status RavenDB answered with.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("RavenDB answered HTTP %d: %s", e.StatusCode, e.Message)
}

// IsRejected reports whether RavenDB refused the request itself, so sending it again unchanged cannot succeed.
func IsRejected(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	code := apiErr.StatusCode
	return code >= 400 && code < 500 && code != http.StatusRequestTimeout && code != http.StatusTooManyRequests
}

func hasStatus(err error, code int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == code
}

// do sends the request and decodes a successful JSON answer into out. Changes carry a fresh raft-request-id,
// which RavenDB uses to deduplicate cluster commands.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	if query == nil {
		query = url.Values{}
	}
	if method != http.MethodGet {
		query.Set("raft-request-id", uuid.NewString())
	}
	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}

	resp, err := c.send(ctx, method, target, payload)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	answer, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Message: errorMessage(answer)}
	}
	if out == nil || len(answer) == 0 {
		return nil
	}
	return json.Unmarshal(answer, out)
}

// send tries the nodes in order and returns the first response, moving on only when a node cannot be reached.
func (c *Client) send(ctx context.Context, method, target string, payload []byte) (*http.Response, error) {
	unreachable := make([]error, 0, len(c.urls))
	for _, base := range c.urls {
		req, err := http.NewRequestWithContext(ctx, method, base+target, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err == nil {
			return resp, nil
		}
		unreachable = append(unreachable, err)
	}
	return nil, fmt.Errorf("no RavenDB node reachable: %w", errors.Join(unreachable...))
}

// errorMessage extracts the message from RavenDB's JSON error body, without the stack trace.
func errorMessage(body []byte) string {
	var ravenErr struct{ Message string }
	if json.Unmarshal(body, &ravenErr) == nil && ravenErr.Message != "" {
		return ravenErr.Message
	}
	const limit = 300
	if len(body) > limit {
		return string(body[:limit])
	}
	return string(body)
}
