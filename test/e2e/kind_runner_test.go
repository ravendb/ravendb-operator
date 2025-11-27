package e2e

import (
	"fmt"
	"os"

	"testing"
	"time"

	testutil "ravendb-operator/test/utils"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

var (
	testenv               env.Environment
	clusterName           = "ravendb"
	TestNS                = "ravendb"
	operatorNS            = "ravendb-operator-system"
	operatorImage         = testutil.Getenv("RAVEN_OPERATOR_IMAGE", "thegoldenplatypus/ravendb-operator-multi-node:latest") //todo: change to ravendb hosted image once we restructure
	installMode           = testutil.Getenv("RAVEN_E2E_INSTALL_MODE", "kustomize")
	helmChartPath         = testutil.Getenv("RAVEN_E2E_HELM_CHART_PATH", "chart")
	helmRelease           = testutil.Getenv("RAVEN_E2E_HELM_RELEASE", "ravendb-operator")
	ctlMgrName            = "ravendb-operator-controller-manager"
	certManagerNS         = "cert-manager"
	controllerNS          = "controller"
	metalLBNS             = "metallb-system"
	cmCaInjectorName      = "cert-manager-cainjector"
	cmWebhookName         = "cert-manager-webhook"
	webhookCertName       = "webhook-server-cert"
	crdName               = "ravendbclusters.ravendb.ravendb.io"
	timeout               = 240 * time.Second
	certManagerFilePath   = "https://github.com/cert-manager/cert-manager/releases/download/v1.14.4/cert-manager.yaml"
	localPathFilePath     = "https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.26/deploy/local-path-storage.yaml"
	metallbFilePath       = "https://raw.githubusercontent.com/metallb/metallb/v0.14.3/config/manifests/metallb-native.yaml"
	metallbConfigFilePath = "test/e2e/manifests/metallb-config.yaml"
	nginxIngressFilePath  = "test/e2e/manifests/nginx-ingress-ravendb.yaml"
	crdBasePath           = "config/crd/bases"
	crdDefaultPath        = "config/default"
	rbacPath              = "config/rbac"
	dockerfileName        = "Dockerfile"
)

/*
// to export:
export PROJECT_ROOT="/ravendb-operator"
export E2E_LICENSE_PATH="/ravendb-operator-e2e/license.json"
export E2E_CLIENT_PFX_PATH="/ravendb-operator-e2e/setup_package/admin.client.certificate.ravendb-operator-e2e.pfx"
export E2E_NODE_A_PFX_PATH="/ravendb-operator-e2e/setup_package/A/cluster.server.certificate.ravendb-operator-e2e.pfx"
export E2E_NODE_B_PFX_PATH="/ravendb-operator-e2e/setup_package/B/cluster.server.certificate.ravendb-operator-e2e.pfx"
export E2E_NODE_C_PFX_PATH="/ravendb-operator-e2e/setup_package/C/cluster.server.certificate.ravendb-operator-e2e.pfx"

for /etc/hosts
172.19.255.200 a.ravendb-operator-e2e.ravendb.run a-tcp.ravendb-operator-e2e.ravendb.run
172.19.255.200 b.ravendb-operator-e2e.ravendb.run b-tcp.ravendb-operator-e2e.ravendb.run
172.19.255.200 c.ravendb-operator-e2e.ravendb.run c-tcp.ravendb-operator-e2e.ravendb.run
*/

func TestMain(m *testing.M) {
	cfg := envconf.New()
	testenv = env.NewWithConfig(cfg)

	setup := []env.Func{

		envfuncs.CreateKindCluster(clusterName),
		testutil.BindKubectlToSuiteEnv(),
		testutil.ApplyManifest(certManagerFilePath),
		testutil.ApplyManifest(localPathFilePath),
		testutil.ApplyManifest(metallbFilePath),
		testutil.WaitForDeployment(controllerNS, metalLBNS, timeout),

		testutil.ApplyManifest(metallbConfigFilePath),
		testutil.ApplyManifest(nginxIngressFilePath),

		testutil.WaitForIngressControllerReady(timeout),
		testutil.WaitForIngressAdmissionReady(timeout),
		testutil.WaitForDeployment(certManagerNS, certManagerNS, timeout),
		testutil.WaitForDeployment(cmCaInjectorName, certManagerNS, timeout),
		testutil.WaitForDeployment(cmWebhookName, certManagerNS, timeout),

		testutil.ApplyCRDsFromDir(crdBasePath),
		testutil.WaitForCRDEstablished(crdName, timeout),

		testutil.BuildAndLoadOperator(operatorImage, dockerfileName, testutil.RepoRoot()),
	}

	if installMode == "helm" {
		fmt.Println("[e2e] install mode: HELM (InstallOperatorHelm)")
		setup = append(setup, testutil.InstallOperatorHelm(helmRelease, operatorNS, helmChartPath, timeout))
	} else {
		fmt.Println("[e2e] install mode: KUSTOMIZE (ApplyKustomize)")
		setup = append(setup, envfuncs.CreateNamespace(TestNS))
		setup = append(setup, testutil.ApplyKustomize(crdDefaultPath))
	}
	setup = append(setup,
		testutil.WaitForSecret(webhookCertName, operatorNS, timeout),
		testutil.SetDeploymentImage(operatorNS, ctlMgrName, "manager", operatorImage),
		testutil.PatchImagePullPolicyIfNotPresent(operatorNS, ctlMgrName),
		testutil.DumpDeploymentImage(operatorNS, ctlMgrName),
		testutil.WaitForDeployment(ctlMgrName, operatorNS, timeout),
	)

	testenv.Setup(setup...)

	testenv.Finish(
		envfuncs.DestroyKindCluster(clusterName),
	)

	os.Exit(testenv.Run(m))
}
