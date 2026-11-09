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

package clientcert

import (
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestIssue_ClientCertificateInPasswordlessPFX(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	cert, err := Issue("shop/orders-api", 90*24*time.Hour, now)
	require.NoError(t, err)

	key, x509Cert, _, err := pkcs12.DecodeChain(cert.PFX, "")
	require.NoError(t, err, "the PFX must open without a password")
	require.Equal(t, cert.X509.Raw, x509Cert.Raw)
	require.True(t, key.(*rsa.PrivateKey).PublicKey.Equal(x509Cert.PublicKey), "the PFX key belongs to the certificate")

	require.Equal(t, "shop/orders-api", x509Cert.Subject.CommonName)
	require.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}, x509Cert.ExtKeyUsage)
	require.False(t, x509Cert.IsCA)
	require.Equal(t, now.Add(90*24*time.Hour), x509Cert.NotAfter)
	require.True(t, x509Cert.NotBefore.Before(now), "NotBefore is backdated for clock skew")
}

func TestIssue_EveryCertificateHasItsOwnKey(t *testing.T) {
	now := time.Now()
	first, err := Issue("a", time.Hour, now)
	require.NoError(t, err)
	second, err := Issue("a", time.Hour, now)
	require.NoError(t, err)

	require.NotEqual(t, first.Thumbprint(), second.Thumbprint())
	require.NotEqual(t, first.X509.PublicKey, second.X509.PublicKey)
}

func TestDecode_RoundTripsIssue(t *testing.T) {
	issued, err := Issue("a", time.Hour, time.Now())
	require.NoError(t, err)

	decoded, err := Decode(issued.PFX)
	require.NoError(t, err)
	require.Equal(t, issued.Thumbprint(), decoded.Thumbprint())

	_, err = Decode([]byte("not a pfx"))
	require.Error(t, err)
	_, err = Decode(nil)
	require.Error(t, err)
}

func TestThumbprint_IsUppercaseSHA1OfDER(t *testing.T) {
	cert, err := Issue("a", time.Hour, time.Now())
	require.NoError(t, err)

	sum := sha1.Sum(cert.X509.Raw)
	require.Equal(t, strings.ToUpper(hex.EncodeToString(sum[:])), cert.Thumbprint())
	require.Len(t, cert.Thumbprint(), 40)

	der, err := base64.StdEncoding.DecodeString(cert.PublicBase64())
	require.NoError(t, err)
	require.Equal(t, cert.X509.Raw, der, "RavenDB gets the public certificate only")
}

func TestRenewAt_TwoThirdsOfTheShorterLifetime(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cert, err := Issue("a", 3000*time.Hour, now)
	require.NoError(t, err)
	notBefore := cert.X509.NotBefore

	lifetime := cert.X509.NotAfter.Sub(notBefore)
	require.Equal(t, notBefore.Add(2000*time.Hour), cert.RenewAt(3000*time.Hour), "the validity it was issued with")
	require.Equal(t, notBefore.Add(200*time.Hour), cert.RenewAt(300*time.Hour), "a validity shortened since issuing")
	require.Equal(t, notBefore.Add(lifetime-lifetime/3), cert.RenewAt(6000*time.Hour),
		"a longer validity waits for the next certificate")

	long, err := Issue("a", 1_300_000*time.Hour, now)
	require.NoError(t, err)
	require.True(t, long.RenewAt(1_300_000*time.Hour).After(now), "a lifetime over a century still renews in the future")
}
