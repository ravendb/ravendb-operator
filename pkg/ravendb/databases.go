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
	"net/http"
	"net/url"
	"strconv"
)

const databasesPath = "/admin/databases"

// DatabaseExists reports whether the cluster holds a database record with this name.
func (c *Client) DatabaseExists(ctx context.Context, name string) (bool, error) {
	err := c.do(ctx, http.MethodGet, databasesPath, url.Values{"name": {name}}, nil, nil)
	if hasStatus(err, http.StatusNotFound) {
		return false, nil
	}
	return err == nil, err
}

// CreateDatabase creates the database on as many nodes as replicationFactor, chosen by RavenDB. A database that
// already exists counts as created: that is also how RavenDB answers a retry of an attempt whose reply was lost.
func (c *Client) CreateDatabase(ctx context.Context, name string, replicationFactor int32) error {
	query := url.Values{
		"name":              {name},
		"replicationFactor": {strconv.Itoa(int(replicationFactor))},
	}
	record := map[string]any{"DatabaseName": name}
	err := c.do(ctx, http.MethodPut, databasesPath, query, record, nil)
	if hasStatus(err, http.StatusConflict) {
		return nil
	}
	return err
}

// DeleteDatabase removes the database and its data from every node.
func (c *Client) DeleteDatabase(ctx context.Context, name string) error {
	params := map[string]any{
		"DatabaseNames": []string{name},
		"HardDelete":    true,
	}
	return c.do(ctx, http.MethodDelete, databasesPath, nil, params, nil)
}
