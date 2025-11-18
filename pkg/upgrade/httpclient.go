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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/pkcs12"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ravendbv1 "ravendb-operator/api/v1"
)

const (
	clientPFXKey = "client.pfx"
	clientPwdKey = "password"
	caCRTKey     = "ca.crt"
)

func BuildHTTPSClientFromCluster(ctx context.Context, kc client.Client, c *ravendbv1.RavenDBCluster) (*http.Client, error) {

	needCA, err := needsCA(c)
	if err != nil {
		return nil, err
	}

	var clientSecret corev1.Secret
	if err := kc.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: c.Spec.ClientCertSecretRef}, &clientSecret); err != nil {
		return nil, fmt.Errorf("get client cert secret %q: %w", c.Spec.ClientCertSecretRef, err)
	}

	pair, err := loadClientPair(clientSecret)
	if err != nil {
		return nil, err
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{pair},
	}

	if needCA {
		pool, err := loadCAPool(ctx, kc, c)
		if err != nil {
			return nil, err
		}
		tlsCfg.RootCAs = pool
	}

	tr := &http.Transport{TLSClientConfig: tlsCfg}
	return &http.Client{Transport: tr}, nil
}

func needsCA(c *ravendbv1.RavenDBCluster) (bool, error) {
	switch c.Spec.Mode {
	case ravendbv1.ModeLetsEncrypt:
		return false, nil
	case ravendbv1.ModeNone: // self-signed
		return true, nil
	default:
		return false, fmt.Errorf("unsupported mode: %q", c.Spec.Mode)
	}
}

func pfxToTLSCert(pfx []byte, password string) (tls.Certificate, error) {

	blocks, err := pkcs12.ToPEM(pfx, password)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("decode client.pfx: %w", err)
	}
	var certPEM, keyPEM []byte
	for _, b := range blocks {
		if strings.Contains(b.Type, "PRIVATE KEY") {
			keyPEM = append(keyPEM, pem.EncodeToMemory(b)...)
		} else {
			certPEM = append(certPEM, pem.EncodeToMemory(b)...)
		}
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

func loadClientPair(secret corev1.Secret) (tls.Certificate, error) {
	pfx, ok := secret.Data[clientPFXKey]
	if !ok || len(pfx) == 0 {
		return tls.Certificate{}, fmt.Errorf("client secret %q missing %q", secret.GetName(), clientPFXKey)
	}
	pass := string(secret.Data[clientPwdKey]) // allow empty password
	return pfxToTLSCert(pfx, pass)
}

func loadCAPool(ctx context.Context, kc client.Client, c *ravendbv1.RavenDBCluster) (*x509.CertPool, error) {
	caName := strings.TrimSpace(*c.Spec.CACertSecretRef)

	var caSecret corev1.Secret
	if err := kc.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: caName}, &caSecret); err != nil {
		return nil, fmt.Errorf("get CA secret %q: %w", caName, err)
	}

	caPEM, ok := caSecret.Data[caCRTKey]
	if !ok || len(caPEM) == 0 {
		return nil, fmt.Errorf("CA secret %q missing %q", caName, caCRTKey)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("failed to parse %q from %q", caCRTKey, caName)
	}

	return pool, nil
}
