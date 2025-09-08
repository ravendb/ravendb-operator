package v1alpha1

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func now() metav1.Time { return metav1.NewTime(time.Unix(1, 0)) }

func newCluster(withExternal bool) *RavenDBCluster {
	c := &RavenDBCluster{}
	if withExternal {
		c.Spec.ExternalAccessConfiguration = &ExternalAccessConfiguration{
			Type: ExternalAccessTypeIngressController,
		}
	}
	return c
}

func setTrue(c *RavenDBCluster, t ClusterConditionType) {
	c.SetConditionTrue(t, ReasonCompleted, "ok", now())
}

func setFalse(c *RavenDBCluster, t ClusterConditionType, reason ClusterConditionReason, msg string) {
	c.SetConditionFalse(t, reason, msg, now())
}

func assertReadyFalseWithReason(t *testing.T, c *RavenDBCluster, wantReason ClusterConditionType) {
	ready := getReadyCondition(t, c)
	require.Equal(t, metav1.ConditionFalse, ready.Status)
	require.Equal(t, string(wantReason), ready.Reason)
}

func getReadyCondition(t *testing.T, c *RavenDBCluster) metav1.Condition {
	t.Helper()
	for i := range c.Status.Conditions {
		if c.Status.Conditions[i].Type == string(ConditionReady) {
			return c.Status.Conditions[i]
		}
	}
	t.Fatalf("Ready condition not found")
	return metav1.Condition{}
}

func Test_TL1_AllRequiredTrue(t *testing.T) {
	c := newCluster(true)
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setTrue(c, ConditionStorageReady)
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)
	setTrue(c, ConditionExternalAccessReady)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()

	ready := getReadyCondition(t, c)
	require.Equal(t, metav1.ConditionTrue, ready.Status)
	require.Equal(t, string(ReasonCompleted), ready.Reason)
	require.Equal(t, "Cluster is ready", c.Status.Message)
	require.Equal(t, PhaseRunning, c.Status.Phase)
}

func Test_TL2_StorageFalse_OthersTrue(t *testing.T) {
	c := newCluster(true)
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setFalse(c, ConditionStorageReady, ReasonPVCNotBound, "PVCs not bound: ns/pvc-1")
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)
	setTrue(c, ConditionExternalAccessReady)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()

	assertReadyFalseWithReason(t, c, ConditionStorageReady)
	require.Equal(t, "PVCNotBound: PVCs not bound: ns/pvc-1", c.Status.Message)
	require.Equal(t, PhaseDeploying, c.Status.Phase)
}

func Test_TL3_NodesHealthyFalse_OthersTrue(t *testing.T) {
	c := newCluster(true)
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setTrue(c, ConditionStorageReady)
	setFalse(c, ConditionNodesHealthy, ReasonPodsNotReady, "pods failed: ns/p")
	setTrue(c, ConditionBootstrapCompleted)
	setTrue(c, ConditionExternalAccessReady)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	assertReadyFalseWithReason(t, c, ConditionNodesHealthy)
	require.Equal(t, "PodsNotReady: pods failed: ns/p", c.Status.Message)
	require.Equal(t, PhaseDeploying, c.Status.Phase)
}

func Test_TL4_CertsFalse_OthersTrue(t *testing.T) {
	c := newCluster(true)
	setFalse(c, ConditionCertificatesReady, ReasonCertSecretMissing, "missing certs")
	setTrue(c, ConditionLicensesValid)
	setTrue(c, ConditionStorageReady)
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)
	setTrue(c, ConditionExternalAccessReady)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	assertReadyFalseWithReason(t, c, ConditionCertificatesReady)
	require.Equal(t, "CertSecretMissing: missing certs", c.Status.Message)
	require.Equal(t, PhaseDeploying, c.Status.Phase)
}

func Test_TL5_LicenseFalse_OthersTrue(t *testing.T) {
	c := newCluster(true)
	setTrue(c, ConditionCertificatesReady)
	setFalse(c, ConditionLicensesValid, ReasonLicenseSecretMissing, "missing lic")
	setTrue(c, ConditionStorageReady)
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)
	setTrue(c, ConditionExternalAccessReady)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	assertReadyFalseWithReason(t, c, ConditionLicensesValid)
	require.Equal(t, "LicenseSecretMissing: missing lic", c.Status.Message)
	require.Equal(t, PhaseDeploying, c.Status.Phase)
}

func Test_TL6_BootstrapFalse_OthersTrue(t *testing.T) {
	c := newCluster(true)
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setTrue(c, ConditionStorageReady)
	setTrue(c, ConditionNodesHealthy)
	setFalse(c, ConditionBootstrapCompleted, ReasonBootstrapJobRunning, "bootstrap job still running")
	setTrue(c, ConditionExternalAccessReady)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	assertReadyFalseWithReason(t, c, ConditionBootstrapCompleted)
	require.Equal(t, "BootstrapJobRunning: bootstrap job still running", c.Status.Message)
	require.Equal(t, PhaseDeploying, c.Status.Phase)
}

func Test_TL7_ExternalConfigured_False_OthersTrue(t *testing.T) {
	c := newCluster(true)
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setTrue(c, ConditionStorageReady)
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)
	setFalse(c, ConditionExternalAccessReady, ReasonIngressPendingAddress, "waiting for ingress lb")

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	assertReadyFalseWithReason(t, c, ConditionExternalAccessReady)
	require.Equal(t, "IngressPendingAddress: waiting for ingress lb", c.Status.Message)
	require.Equal(t, PhaseDeploying, c.Status.Phase)
}

func Test_TL8_ExternalNotConfigured_Ignored_AllOthersTrue(t *testing.T) {
	c := newCluster(false) // no external access
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setTrue(c, ConditionStorageReady)
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	ready := getReadyCondition(t, c)
	require.Equal(t, metav1.ConditionTrue, ready.Status)
	require.Equal(t, "Cluster is ready", c.Status.Message)
	require.Equal(t, PhaseRunning, c.Status.Phase)
}

func Test_TL9_MultipleFalse_FirstByOrderWins(t *testing.T) {
	c := newCluster(false)
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setFalse(c, ConditionStorageReady, ReasonPVCNotBound, "pvc issue")
	setFalse(c, ConditionNodesHealthy, ReasonPodsNotReady, "pods bad")
	setTrue(c, ConditionBootstrapCompleted)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	assertReadyFalseWithReason(t, c, ConditionStorageReady)
	require.Equal(t, "PVCNotBound: pvc issue", c.Status.Message)
	require.Equal(t, PhaseDeploying, c.Status.Phase)
}

func Test_TL10_MissingCondition_TriggersNotSatisfiedFallback(t *testing.T) {
	c := newCluster(false)
	setTrue(c, ConditionCertificatesReady)
	// LicensesValid missing
	setTrue(c, ConditionStorageReady)
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	assertReadyFalseWithReason(t, c, ConditionLicensesValid)
	require.Equal(t, "LicensesValid not satisfied", c.Status.Message)
	require.Equal(t, PhaseDeploying, c.Status.Phase)
}

// "Progressing=True" should not override Ready=True for Phase
func Test_TL11_ProgressingTrue_but_AllRequiredTrue(t *testing.T) {
	c := newCluster(false)
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setTrue(c, ConditionStorageReady)
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)
	setTrue(c, ConditionProgressing)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	ready := getReadyCondition(t, c)
	require.Equal(t, metav1.ConditionTrue, ready.Status)
	require.Equal(t, PhaseRunning, c.Status.Phase)
}

func Test_TL12_DegradedTrue_while_NotAllRequiredTrue(t *testing.T) {
	c := newCluster(false)
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setFalse(c, ConditionStorageReady, ReasonPVCNotBound, "pvc issue")
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)
	setTrue(c, ConditionDegraded)

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	assertReadyFalseWithReason(t, c, ConditionStorageReady)
	require.Equal(t, PhaseError, c.Status.Phase) // degraded drives Error when not Ready
}

func Test_TL13_DegradedTrue_and_AllRequiredTrue(t *testing.T) {
	c := newCluster(false)
	setTrue(c, ConditionCertificatesReady)
	setTrue(c, ConditionLicensesValid)
	setTrue(c, ConditionStorageReady)
	setTrue(c, ConditionNodesHealthy)
	setTrue(c, ConditionBootstrapCompleted)
	setTrue(c, ConditionDegraded) // Ready > Progressing > Degraded

	c.ComputeReady(now())
	c.UpdatePhaseFromConditions()
	ready := getReadyCondition(t, c)
	require.Equal(t, metav1.ConditionTrue, ready.Status)
	require.Equal(t, PhaseRunning, c.Status.Phase)
}
