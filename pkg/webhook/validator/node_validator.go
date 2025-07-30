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
	"net"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type nodeValidator struct {
	client client.Reader
}

func NewNodeValidator(c client.Reader) *nodeValidator {
	return &nodeValidator{client: c}
}

func (v *nodeValidator) Name() string {
	return "node-validator"
}

type nodeInput struct {
	Index      int
	Tag        string
	PublicUrl  string
	TcpUrl     string
	CertSecret string
}

func (v *nodeValidator) ValidateCreate(ctx context.Context, c ClusterAdapter) error {
	var errs []string
	var input []nodeInput

	tags := c.GetNodeTags()
	pubUrls := c.GetNodePublicUrls()
	tcpUrls := c.GetNodeTcpUrls()
	certRefs := c.GetNodeCertSecretRefs()
	mode := c.GetMode()
	domain := c.GetDomain()
	extAccType := c.GetExternalAccessType()

	for i := range tags {
		var cert string
		if certRefs[i] != nil {
			cert = *certRefs[i]
		}
		input = append(input, nodeInput{
			Index:      i,
			Tag:        tags[i],
			PublicUrl:  pubUrls[i],
			TcpUrl:     tcpUrls[i],
			CertSecret: cert,
		})
	}
	errs = append(errs, ValidateNodesNotEmpty(tags)...)
	errs = append(errs, ValidateUniqueTags(tags)...)
	errs = append(errs, ValidateUniqueUrls(pubUrls, tcpUrls)...)
	errs = append(errs, ValidatePortsConsistency(pubUrls, tcpUrls, extAccType)...)

	for _, n := range input {
		errs = append(errs, ValidateNodeUrl(n.Tag, n.PublicUrl, domain, "https", "publicServerUrl", n.Tag+".")...)
		errs = append(errs, ValidateNodeUrl(n.Tag, n.TcpUrl, domain, "tcp", "publicServerUrlTcp", n.Tag+"-tcp.")...)
		errs = append(errs, ValidateNodeCertSecret(ctx, v, mode, n.Tag, n.CertSecret)...)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "\n"))
	}
	return nil
}

func (v *nodeValidator) ValidateUpdate(ctx context.Context, _, newC ClusterAdapter) error {
	return v.ValidateCreate(ctx, newC)
}

func ValidateNodesNotEmpty(tags []string) []string {
	if len(tags) == 0 {
		return []string{"spec.nodes must contain at least one node"}
	}
	return nil
}

func ValidateUniqueTags(tags []string) []string {
	var errs []string
	seen := map[string]bool{}

	for _, tag := range tags {
		if seen[tag] {
			errs = append(errs, fmt.Sprintf("spec.nodes: duplicate tag '%s'", tag))
		}
		seen[tag] = true
	}

	return errs
}

func ValidateUniqueUrls(publicUrls, tcpUrls []string) []string {
	var errs []string
	seen := map[string]string{}

	for i, url := range publicUrls {
		label := fmt.Sprintf("spec.nodes[%d].publicServerUrl", i)
		stored, exists := seen[url]
		if exists {
			errs = append(errs, fmt.Sprintf("%s duplicates URL already used in %s", label, stored))
		} else {
			seen[url] = label
		}
	}

	for i, url := range tcpUrls {
		label := fmt.Sprintf("spec.nodes[%d].publicServerUrlTcp", i)
		stored, exists := seen[url]
		if exists {
			errs = append(errs, fmt.Sprintf("%s duplicates URL already used in %s", label, stored))
		} else {
			seen[url] = label
		}
	}

	return errs
}

func ValidatePortsConsistency(publicUrls, tcpUrls []string, extAccType string) []string {
	var expectedPort string

	if extAccType != "ingress-controller" {
		return nil
	}

	for i := range publicUrls {
		pubPort := extractPort(publicUrls[i])
		tcpPort := extractPort(tcpUrls[i])

		if pubPort != tcpPort {
			return []string{"spec.nodes: publicServerUrl and publicServerUrlTcp ports must match"}
		}

		if i == 0 {
			expectedPort = pubPort
		} else if pubPort != expectedPort {
			return []string{"spec.nodes: ports must be consistent across all nodes"}
		}
	}

	return nil
}

func ValidateNodeUrl(tag, rawUrl, domain, expectedScheme, labelPrefix, expectedHostPrefix string) []string {
	var errs []string
	label := fmt.Sprintf("spec.nodes[%s].%s", tag, labelPrefix)

	u, err := url.Parse(rawUrl)
	if err != nil {
		return []string{fmt.Sprintf("%s: invalid URL format", label)}
	}

	if u.Scheme != expectedScheme {
		errs = append(errs, fmt.Sprintf("%s: scheme must be '%s'", label, expectedScheme))
	}

	host := u.Hostname()

	if !strings.HasPrefix(host, expectedHostPrefix) {
		errs = append(errs, fmt.Sprintf("%s: hostname must start with '%s'", label, expectedHostPrefix))
	}

	if !strings.HasSuffix(host, domain) {
		errs = append(errs, fmt.Sprintf("%s: hostname must be subdomain of '%s'", label, domain))
	}

	if _, port, err := net.SplitHostPort(u.Host); err != nil || port == "" {
		errs = append(errs, fmt.Sprintf("%s: must include a port", label))
	}

	if u.Path != "" && u.Path != "/" {
		errs = append(errs, fmt.Sprintf("%s: must not contain path", label))
	}
	if u.RawQuery != "" {
		errs = append(errs, fmt.Sprintf("%s: must not contain query", label))
	}
	if u.Fragment != "" {
		errs = append(errs, fmt.Sprintf("%s: must not contain fragment", label))
	}

	return errs
}

func ValidateNodeCertSecret(ctx context.Context, v *nodeValidator, mode, tag, secretName string) []string {
	var errs []string
	label := fmt.Sprintf("spec.nodes[tag=%s].certsSecretRef", tag)

	if mode == "LetsEncrypt" && secretName == "" {
		errs = append(errs, fmt.Sprintf("%s is required when mode is LetsEncrypt", label))
	}
	if mode == "None" && secretName != "" {
		errs = append(errs, fmt.Sprintf("%s must not be set when mode is None", label))
	}

	if secretName == "" {
		return errs
	}

	secret, err := v.getSecret(ctx, secretName)
	if err != nil {
		errs = append(errs, fmt.Sprintf("%s: %v", label, err))
		return errs
	}

	if len(secret.Data) != 1 {
		errs = append(errs, fmt.Sprintf("%s: must contain exactly one .pfx file", label))
		return errs
	}

	for key := range secret.Data {
		if !strings.HasSuffix(key, ".pfx") {
			errs = append(errs, fmt.Sprintf("%s: file '%s' must end with .pfx", label, key))
		}
		break
	}

	return errs
}

func extractPort(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		return ""
	}
	_, port, _ := net.SplitHostPort(host)
	return port
}

func (v *nodeValidator) getSecret(ctx context.Context, name string) (*corev1.Secret, error) {
	var secret corev1.Secret
	err := v.client.Get(ctx, client.ObjectKey{Name: name, Namespace: "ravendb"}, &secret)
	if err != nil {
		return nil, fmt.Errorf("secret '%s' not found", name)
	}
	return &secret, nil
}
