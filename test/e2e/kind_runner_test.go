package e2e

import (
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
	operatorImage         = "thegoldenplatypus/ravendb-operator-multi-node:latest" //todo: change to ravendb hosted image once we restructure
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
	certHookPath          = "config/cert-hook"
	bootstrapperHookPath  = "config/bootstrapper-hook"
	rbacPath              = "config/rbac"
	dockerfileName        = "Dockerfile"
)

/*
// to export:
export PROJECT_ROOT="/ravendb-operator"
export E2E_LICENSE_PATH="/ravendb-e2e/license.json"
export E2E_CLIENT_PFX_PATH="/ravendb-e2e/setup_package/admin.client.certificate.ravendbe2e.pfx"
export E2E_NODE_A_PFX_PATH="/ravendb-e2e/setup_package/A/cluster.server.certificate.ravendbe2e.pfx"
export E2E_NODE_B_PFX_PATH="/ravendb-e2e/setup_package/B/cluster.server.certificate.ravendbe2e.pfx"
export E2E_NODE_C_PFX_PATH="/ravendb-e2e/setup_package/C/cluster.server.certificate.ravendbe2e.pfx"

for /etc/hosts
todo: put setup_package to the repo for the CI
172.19.255.200 a.ravendbe2e.development.run a-tcp.ravendbe2e.development.run
172.19.255.200 b.ravendbe2e.development.run b-tcp.ravendbe2e.development.run
172.19.255.200 c.ravendbe2e.development.run c-tcp.ravendbe2e.development.run
*/

func TestMain(m *testing.M) {
	cfg := envconf.New()
	testenv = env.NewWithConfig(cfg)

	testenv.Setup(

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
		testutil.ApplyKustomize(crdDefaultPath),

		envfuncs.CreateNamespace(TestNS),
		testutil.ApplyKustomize(certHookPath),
		testutil.ApplyKustomize(bootstrapperHookPath),

		testutil.WaitForSecret(webhookCertName, operatorNS, timeout),
		testutil.SetDeploymentImage(operatorNS, ctlMgrName, "manager", operatorImage),
		testutil.WaitForDeployment(ctlMgrName, operatorNS, timeout),
	)

	testenv.Finish(
		envfuncs.DestroyKindCluster(clusterName),
	)

	os.Exit(testenv.Run(m))
}
