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
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// recorded is one request as RavenDB would see it.
type recorded struct {
	method string
	path   string
	query  map[string]string
	body   map[string]any
}

// recordingServer answers every request with status and answer, and records what it got.
func recordingServer(t *testing.T, status int, answer string) (*Client, *[]recorded) {
	t.Helper()
	var got []recorded
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req := recorded{method: r.Method, path: r.URL.Path, query: map[string]string{}}
		for k := range r.URL.Query() {
			req.query[k] = r.URL.Query().Get(k)
		}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			if err := json.Unmarshal(raw, &req.body); err != nil {
				t.Errorf("request body is not JSON: %v", err)
			}
		}
		got = append(got, req)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.Client(), srv.URL), &got
}

func TestRequests(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		call   func(*Client) error
		method string
		path   string
		query  map[string]string
		body   map[string]any
	}{
		"database exists": {
			call:   func(rc *Client) error { _, err := rc.DatabaseExists(ctx, "Orders"); return err },
			method: http.MethodGet, path: "/admin/databases", query: map[string]string{"name": "Orders"},
		},
		"create database": {
			call:   func(rc *Client) error { return rc.CreateDatabase(ctx, "Orders", 3) },
			method: http.MethodPut, path: "/admin/databases",
			query: map[string]string{"name": "Orders", "replicationFactor": "3"},
			body:  map[string]any{"DatabaseName": "Orders"},
		},
		"delete database with its data": {
			call:   func(rc *Client) error { return rc.DeleteDatabase(ctx, "Orders") },
			method: http.MethodDelete, path: "/admin/databases",
			body: map[string]any{"DatabaseNames": []any{"Orders"}, "HardDelete": true},
		},
		"list certificates": {
			call:   func(rc *Client) error { _, err := rc.Certificates(ctx); return err },
			method: http.MethodGet, path: "/admin/certificates", query: map[string]string{"metadataOnly": "true"},
		},
		"register certificate, permissions never null": {
			call: func(rc *Client) error {
				return rc.RegisterCertificate(ctx, Certificate{
					Name: "shop/ops", Certificate: "QUJD", SecurityClearance: "Operator",
				})
			},
			method: http.MethodPut, path: "/admin/certificates",
			body: map[string]any{
				"Name": "shop/ops", "Certificate": "QUJD", "SecurityClearance": "Operator", "Permissions": map[string]any{},
			},
		},
		"edit certificate": {
			call: func(rc *Client) error {
				return rc.EditCertificate(ctx, Certificate{
					Name: "shop/api", Thumbprint: "AB12", SecurityClearance: "ValidUser",
					Permissions: map[string]string{"Orders": "Read"},
				})
			},
			method: http.MethodPost, path: "/admin/certificates/edit",
			body: map[string]any{
				"Name": "shop/api", "Thumbprint": "AB12", "SecurityClearance": "ValidUser",
				"Permissions": map[string]any{"Orders": "Read"},
			},
		},
		"revoke certificate": {
			call:   func(rc *Client) error { return rc.RevokeCertificate(ctx, "AB12") },
			method: http.MethodDelete, path: "/admin/certificates", query: map[string]string{"thumbprint": "AB12"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			rc, got := recordingServer(t, http.StatusOK, "{}")
			require.NoError(t, tc.call(rc))

			req := (*got)[0]
			require.Equal(t, tc.method, req.method)
			require.Equal(t, tc.path, req.path)
			for k, v := range tc.query {
				require.Equal(t, v, req.query[k], "query %s", k)
			}
			_, hasRaftID := req.query["raft-request-id"]
			require.Equal(t, tc.method != http.MethodGet, hasRaftID, "changes carry a raft-request-id")
			require.Equal(t, tc.body, req.body)
		})
	}
}

func TestAnswers(t *testing.T) {
	ctx := context.Background()

	rc, _ := recordingServer(t, http.StatusNotFound, "")
	exists, err := rc.DatabaseExists(ctx, "Orders")
	require.NoError(t, err)
	require.False(t, exists)

	rc, _ = recordingServer(t, http.StatusConflict, `{"Message":"Database 'Orders' already exists!"}`)
	require.NoError(t, rc.CreateDatabase(ctx, "Orders", 3), "an existing database counts as created")

	rc, _ = recordingServer(t, http.StatusOK, `{"Results":[
		{"Name":"shop/api","Thumbprint":"AB12","SecurityClearance":"ValidUser","Permissions":{"Orders":"ReadWrite"}}
	],"LoadedServerCert":"CD34"}`)
	certs, err := rc.Certificates(ctx)
	require.NoError(t, err)
	require.Equal(t, []Certificate{{
		Name: "shop/api", Thumbprint: "AB12", SecurityClearance: "ValidUser",
		Permissions: map[string]string{"Orders": "ReadWrite"},
	}}, certs)
}

func TestErrors_CarryRavenDBMessageAndClassify(t *testing.T) {
	rc, _ := recordingServer(t, http.StatusBadRequest,
		`{"Type":"System.ArgumentException","Message":"Database name is not valid","Error":"long stack trace"}`)
	err := rc.CreateDatabase(context.Background(), "bad name", 1)

	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	require.Equal(t, "Database name is not valid", apiErr.Message)
	require.True(t, IsRejected(err))

	for status, rejected := range map[int]bool{
		http.StatusForbidden:           true,
		http.StatusRequestTimeout:      false,
		http.StatusTooManyRequests:     false,
		http.StatusServiceUnavailable:  false,
		http.StatusInternalServerError: false,
	} {
		require.Equal(t, rejected, IsRejected(&APIError{StatusCode: status}), "HTTP %d", status)
	}
	require.False(t, IsRejected(errors.New("connection refused")))
}

func TestSend_FallsBackToNextReachableNode(t *testing.T) {
	down, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	downURL := "http://" + down.Addr().String()
	require.NoError(t, down.Close())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	exists, err := NewClient(srv.Client(), downURL, srv.URL).DatabaseExists(context.Background(), "Orders")
	require.NoError(t, err)
	require.True(t, exists)

	_, err = NewClient(srv.Client(), downURL).DatabaseExists(context.Background(), "Orders")
	require.ErrorContains(t, err, "no RavenDB node reachable")
}
