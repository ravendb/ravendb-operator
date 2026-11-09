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

// Package clientcert issues the client certificates applications use to authenticate to RavenDB.
// RavenDB trusts a client certificate by its registered thumbprint, so the certificates are self-signed
// and their private keys never leave the operator and the Secret it writes.
package clientcert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"math/big"
	"strings"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

const (
	keyBits = 2048
	// clockSkew backdates NotBefore so a server whose clock runs slightly behind still accepts a fresh certificate.
	clockSkew = 5 * time.Minute
)

// Certificate is a client certificate and the passwordless PFX that carries its private key.
type Certificate struct {
	X509 *x509.Certificate
	PFX  []byte
}

// Issue creates a client certificate valid for the given duration from now.
func Issue(commonName string, validity time.Duration, now time.Time) (*Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, keyBits)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-clockSkew),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}

	pfx, err := pkcs12.Modern.Encode(key, cert, nil, "")
	if err != nil {
		return nil, err
	}
	return &Certificate{X509: cert, PFX: pfx}, nil
}

// Decode reads a PFX that Issue produced.
func Decode(pfx []byte) (*Certificate, error) {
	_, cert, _, err := pkcs12.DecodeChain(pfx, "")
	if err != nil {
		return nil, err
	}
	return &Certificate{X509: cert, PFX: pfx}, nil
}

// Thumbprint is the uppercase SHA-1 hash of the certificate, which is how RavenDB identifies it.
func (c *Certificate) Thumbprint() string {
	sum := sha1.Sum(c.X509.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// PublicBase64 is the public certificate in the form RavenDB registers.
func (c *Certificate) PublicBase64() string {
	return base64.StdEncoding.EncodeToString(c.X509.Raw)
}

// RenewAt is the moment two thirds of the certificate's lifetime have passed. The lifetime counts as at most
// validity, so shortening the validity brings the renewal of an existing certificate forward.
func (c *Certificate) RenewAt(validity time.Duration) time.Time {
	lifetime := min(c.X509.NotAfter.Sub(c.X509.NotBefore), validity)
	return c.X509.NotBefore.Add(lifetime - lifetime/3)
}
