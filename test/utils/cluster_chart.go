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
package testutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterChartImage               = "ravendb/ravendb:6.2.11-ubuntu.22.04-x64"
	clusterChartMode                = "LetsEncrypt"
	clusterChartEmail               = "user@ravendb.net"
	clusterChartDomain              = "ravendb-operator-e2e.ravendb.run"
	clusterChartStorageSize         = "10Gi"
	clusterChartStorageClassName    = "local-path"
	clusterChartDefaultPathFromRoot = "helm/cluster-chart"
)

type ClusterChartCase struct {
	Name             string
	Namespace        string
	IngressClassName string
	Provision        bool
	ProvisionMixed   bool
	InstallTimeout   time.Duration
}

func InstallClusterChart(t *testing.T, tc ClusterChartCase) (ctrlclient.Client, ctrlclient.ObjectKey) {
	t.Helper()
	cli := K8sClient(t)

	release := SanitizeName("e2e-" + tc.Name)
	timeout := tc.InstallTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var err error
	switch {
	case tc.ProvisionMixed:
		err = helmInstallClusterChartMixed(ctx, release, tc.Namespace, tc.IngressClassName, timeout)
	case tc.Provision:
		err = helmInstallClusterChartProvision(ctx, release, tc.Namespace, tc.IngressClassName, timeout)
	default:
		err = helmInstallClusterChartBYO(ctx, release, tc.Namespace, tc.IngressClassName, timeout)
	}
	require.NoError(t, err)

	k := Key(tc.Namespace, release)
	WaitReadable(t, cli, k, 90*time.Second)
	return cli, k
}

func helmInstallClusterChartProvision(ctx context.Context, release, ns, ingressClassName string, timeout time.Duration) error {
	licensePath := os.Getenv(EnvLicensePath)
	clientPFXPath := os.Getenv(EnvClientPFXPath)
	nodeAPath := os.Getenv(EnvNodeAPFXPath)
	nodeBPath := os.Getenv(EnvNodeBPFXPath)
	nodeCPath := os.Getenv(EnvNodeCPFXPath)

	missing := []string{}
	if licensePath == "" {
		missing = append(missing, EnvLicensePath)
	}
	if clientPFXPath == "" {
		missing = append(missing, EnvClientPFXPath)
	}
	if nodeAPath == "" {
		missing = append(missing, EnvNodeAPFXPath)
	}
	if nodeBPath == "" {
		missing = append(missing, EnvNodeBPFXPath)
	}
	if nodeCPath == "" {
		missing = append(missing, EnvNodeCPFXPath)
	}
	if len(missing) > 0 {
		return fmt.Errorf("InstallClusterChart provisioning mode: missing env vars: %s", strings.Join(missing, ", "))
	}

	args := []string{
		"upgrade", "--install", release, ClusterChartPath(),
		"-n", ns,
		"--create-namespace",
		"--wait",
		"--timeout", timeout.String(),

		"--set-file", "secrets.license.file=" + licensePath,
		"--set-file", "secrets.clientCert.file=" + clientPFXPath,
		"--set-file", "secrets.nodeCerts.files.a=" + nodeAPath,
		"--set-file", "secrets.nodeCerts.files.b=" + nodeBPath,
		"--set-file", "secrets.nodeCerts.files.c=" + nodeCPath,
	}
	args = append(args, commonClusterChartArgs(ingressClassName)...)

	if err := RunHelm(ctx, args...); err != nil {
		return fmt.Errorf("helm install cluster chart (provision): %w", err)
	}
	return nil
}

func helmInstallClusterChartMixed(ctx context.Context, release, ns, ingressClassName string, timeout time.Duration) error {
	licensePath := os.Getenv(EnvLicensePath)
	clientPFXPath := os.Getenv(EnvClientPFXPath)

	missing := []string{}
	if licensePath == "" {
		missing = append(missing, EnvLicensePath)
	}
	if clientPFXPath == "" {
		missing = append(missing, EnvClientPFXPath)
	}
	if len(missing) > 0 {
		return fmt.Errorf("InstallClusterChart mixed mode: missing env vars: %s", strings.Join(missing, ", "))
	}

	args := []string{
		"upgrade", "--install", release, ClusterChartPath(),
		"-n", ns,
		"--create-namespace",
		"--wait",
		"--timeout", timeout.String(),

		"--set-file", "secrets.license.file=" + licensePath,
		"--set-file", "secrets.clientCert.file=" + clientPFXPath,
	}
	args = append(args, commonClusterChartArgs(ingressClassName)...)

	if err := RunHelm(ctx, args...); err != nil {
		return fmt.Errorf("helm install cluster chart (mixed): %w", err)
	}
	return nil
}

func helmInstallClusterChartBYO(ctx context.Context, release, ns, ingressClassName string, timeout time.Duration) error {
	args := []string{
		"upgrade", "--install", release, ClusterChartPath(),
		"-n", ns,
		"--wait",
		"--timeout", timeout.String(),
	}
	args = append(args, commonClusterChartArgs(ingressClassName)...)

	if err := RunHelm(ctx, args...); err != nil {
		return fmt.Errorf("helm install cluster chart (BYO): %w", err)
	}
	return nil
}

func commonClusterChartArgs(ingressClassName string) []string {
	return []string{
		"--set", "spec.image=" + clusterChartImage,
		"--set", "spec.mode=" + clusterChartMode,
		"--set", "spec.email=" + clusterChartEmail,
		"--set", "spec.domain=" + clusterChartDomain,

		"--set", "spec.nodes[0].tag=a",
		"--set", "spec.nodes[0].publicServerUrl=https://a." + clusterChartDomain + ":443",
		"--set", "spec.nodes[0].publicServerUrlTcp=tcp://a-tcp." + clusterChartDomain + ":443",
		"--set", "spec.nodes[1].tag=b",
		"--set", "spec.nodes[1].publicServerUrl=https://b." + clusterChartDomain + ":443",
		"--set", "spec.nodes[1].publicServerUrlTcp=tcp://b-tcp." + clusterChartDomain + ":443",
		"--set", "spec.nodes[2].tag=c",
		"--set", "spec.nodes[2].publicServerUrl=https://c." + clusterChartDomain + ":443",
		"--set", "spec.nodes[2].publicServerUrlTcp=tcp://c-tcp." + clusterChartDomain + ":443",

		"--set", "spec.externalAccessConfiguration.type=ingress-controller",
		"--set", "spec.externalAccessConfiguration.ingressControllerContext.ingressClassName=" + ingressClassName,

		"--set", "spec.storage.data.size=" + clusterChartStorageSize,
		"--set", "spec.storage.data.storageClassName=" + clusterChartStorageClassName,
	}
}

func ClusterChartPath() string {
	if p := os.Getenv("RAVEN_E2E_CLUSTER_CHART_PATH"); p != "" {
		return p
	}
	return PathFromRoot(clusterChartDefaultPathFromRoot)
}
