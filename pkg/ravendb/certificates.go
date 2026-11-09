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
)

const certificatesPath = "/admin/certificates"

// Certificate is a client certificate registration. Certificate holds the base64 DER public certificate and is
// only sent when registering; RavenDB never needs the private key.
type Certificate struct {
	Name              string
	Thumbprint        string `json:",omitempty"`
	Certificate       string `json:",omitempty"`
	SecurityClearance string
	Permissions       map[string]string
}

// Certificates lists the registered certificates without their public keys.
func (c *Client) Certificates(ctx context.Context) ([]Certificate, error) {
	var answer struct{ Results []Certificate }
	err := c.do(ctx, http.MethodGet, certificatesPath, url.Values{"metadataOnly": {"true"}}, nil, &answer)
	return answer.Results, err
}

// RegisterCertificate trusts cert.Certificate with its clearance and permissions.
func (c *Client) RegisterCertificate(ctx context.Context, cert Certificate) error {
	return c.do(ctx, http.MethodPut, certificatesPath, nil, withPermissions(cert), nil)
}

// EditCertificate changes the name, clearance and permissions of the certificate with cert.Thumbprint in place.
func (c *Client) EditCertificate(ctx context.Context, cert Certificate) error {
	return c.do(ctx, http.MethodPost, certificatesPath+"/edit", nil, withPermissions(cert), nil)
}

// RevokeCertificate removes the registration, so the certificate no longer authenticates.
func (c *Client) RevokeCertificate(ctx context.Context, thumbprint string) error {
	return c.do(ctx, http.MethodDelete, certificatesPath, url.Values{"thumbprint": {thumbprint}}, nil, nil)
}

// withPermissions sends an empty permission map rather than null, which RavenDB rejects.
func withPermissions(cert Certificate) Certificate {
	if cert.Permissions == nil {
		cert.Permissions = map[string]string{}
	}
	return cert
}
