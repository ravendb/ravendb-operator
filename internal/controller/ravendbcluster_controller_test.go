package controller

// todo: This for testing the full lifecycle and status behavior of the Kubernetes operator.
// import (
// 	"context"
// 	"fmt"
// 	"time"

// 	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"

// 	. "github.com/onsi/ginkgo/v2"
// 	. "github.com/onsi/gomega"
// 	"sigs.k8s.io/controller-runtime/pkg/client"

// 	appsv1 "k8s.io/api/apps/v1"
// 	corev1 "k8s.io/api/core/v1"
// 	networkingv1 "k8s.io/api/networking/v1"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/types"
// )

// type TestFixture struct {
// 	Ctx         context.Context
// 	Namespace   string
// 	K8sClient   client.Client
// 	BaseCluster *ravendbv1alpha1.RavenDBCluster
// }

// func NewTestFixture(k8sClient client.Client) *TestFixture {
// 	ctx := context.Background()

// 	ns := fmt.Sprintf("test-ns-%d", time.Now().UnixNano())
// 	err := k8sClient.Create(ctx, &corev1.Namespace{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: ns,
// 		},
// 	})
// 	if err != nil {
// 		panic(err)
// 	}

// 	base := &ravendbv1alpha1.RavenDBCluster{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      "test-cluster",
// 			Namespace: ns,
// 		},
// 		Spec: ravendbv1alpha1.RavenDBClusterSpec{
// 			Image:            "ravendb/ravendb:latest",
// 			ImagePullPolicy:  "IfNotPresent",
// 			Mode:             "",
// 			License:          "license",
// 			Domain:           "thegoldenplatypus.development.run",
// 			ServerUrl:        "",
// 			ServerUrlTcp:     "",
// 			StorageSize:      "5Gi",
// 			IngressClassName: "nginx",
// 			Nodes: []ravendbv1alpha1.RavenDBNode{
// 				{
// 					Tag:               "a",
// 					PublicServerUrl:    "",
// 					PublicServerUrlTcp: "",
// 				},
// 				{
// 					Tag:               "b",
// 					PublicServerUrl:    "",
// 					PublicServerUrlTcp: "",
// 				},
// 				{
// 					Tag:               "c",
// 					PublicServerUrl:    "",
// 					PublicServerUrlTcp: "",
// 				},
// 			},
// 		},
// 	}

// 	return &TestFixture{
// 		Ctx:         ctx,
// 		Namespace:   ns,
// 		K8sClient:   k8sClient,
// 		BaseCluster: base,
// 	}
// }

// func (f *TestFixture) Cleanup() {
// 	nsObj := &corev1.Namespace{}
// 	err := f.K8sClient.Get(f.Ctx, client.ObjectKey{Name: f.Namespace}, nsObj)
// 	if err == nil {
// 		_ = f.K8sClient.Delete(f.Ctx, nsObj)
// 	}
// }

// func (f *TestFixture) VerifyResources(instance *ravendbv1alpha1.RavenDBCluster) {
// 	for _, node := range instance.Spec.Nodes {
// 		By(fmt.Sprintf("Waiting for StatefulSet %s", node.Tag))
// 		Eventually(func() bool {
// 			sts := &appsv1.StatefulSet{}
// 			err := f.K8sClient.Get(f.Ctx, types.NamespacedName{
// 				Name: fmt.Sprintf("ravendb-%s", node.Tag), Namespace: f.Namespace}, sts)
// 			return err == nil
// 		}, 10*time.Second, 500*time.Millisecond).Should(BeTrue())

// 		By(fmt.Sprintf("Verifying Service for %s", node.Tag))
// 		svc := &corev1.Service{}
// 		Expect(f.K8sClient.Get(f.Ctx, types.NamespacedName{
// 			Name: fmt.Sprintf("ravendb-%s", node.Tag), Namespace: f.Namespace}, svc)).To(Succeed())

// 		By(fmt.Sprintf("Verifying Ingress for %s", node.Tag))
// 		ing := &networkingv1.Ingress{}
// 		Expect(f.K8sClient.Get(f.Ctx, types.NamespacedName{
// 			Name: fmt.Sprintf("ravendb-%s", node.Tag), Namespace: f.Namespace}, ing)).To(Succeed())
// 	}
// }

// func (f *TestFixture) VerifyClusterStatus(instance *ravendbv1alpha1.RavenDBCluster, expectedNodeCount int) {
// 	Eventually(func() (*ravendbv1alpha1.RavenDBCluster, error) {
// 		updated := &ravendbv1alpha1.RavenDBCluster{}
// 		err := f.K8sClient.Get(f.Ctx, types.NamespacedName{
// 			Name: instance.Name, Namespace: f.Namespace}, updated)
// 		if err != nil {
// 			return nil, err
// 		}
// 		return updated, nil
// 	}, 10*time.Second, 100*time.Millisecond).Should(
// 		SatisfyAll(
// 			WithTransform(func(c *ravendbv1alpha1.RavenDBCluster) ravendbv1alpha1.ClusterPhase { return c.Status.Phase }, Equal("Deploying")),
// 			WithTransform(func(c *ravendbv1alpha1.RavenDBCluster) int { return len(c.Status.Nodes) }, Equal(expectedNodeCount)),
// 			WithTransform(func(c *ravendbv1alpha1.RavenDBCluster) bool {
// 				for _, node := range c.Status.Nodes {
// 					if node.Status != "Created" {
// 						return false
// 					}
// 				}
// 				return true
// 			}, BeTrue()),
// 		))
// }

// var _ = Describe("RavenDBCluster controller (insecure mode)", func() {
// 	var fixture *TestFixture

// 	BeforeEach(func() {
// 		fixture = NewTestFixture(k8sClient)
// 	})

// 	AfterEach(func() {
// 		fixture.Cleanup()
// 	})

// 	It("should reconcile and create resources successfully", func() {

// 		instance := fixture.BaseCluster.DeepCopy()
// 		instance.Spec.Mode = "None"
// 		instance.Spec.ServerUrl = "http://0.0.0.0:8080"
// 		instance.Spec.ServerUrlTcp = "tcp://0.0.0.0:38888"
// 		for i, node := range instance.Spec.Nodes {
// 			node.PublicServerUrl = fmt.Sprintf("http://%s.thegoldenplatypus.development.run:8080", node.Tag)
// 			node.PublicServerUrlTcp = fmt.Sprintf("tcp://%s-tcp.thegoldenplatypus.development.run:38888", node.Tag)
// 			node.CertsSecretRef = fmt.Sprintf("ravendb-certs-%s", node.Tag)
// 			instance.Spec.Nodes[i] = node
// 		}

// 		By("Creating RavenDBCluster CR")
// 		Expect(fixture.K8sClient.Create(fixture.Ctx, instance)).To(Succeed())

// 		By("Verifying resources and status")
// 		fixture.VerifyResources(instance)
// 		fixture.VerifyClusterStatus(instance, 3)
// 	})
// })

// var _ = Describe("RavenDBCluster controller (secure mode)", func() {
// 	var fixture *TestFixture

// 	BeforeEach(func() {
// 		fixture = NewTestFixture(k8sClient)
// 	})

// 	AfterEach(func() {
// 		fixture.Cleanup()
// 	})

// 	It("should reconcile and create resources for secure cluster", func() {
// 		By("Creating required cert secrets")
// 		for _, node := range []string{"a", "b", "c"} {
// 			secret := &corev1.Secret{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Name:      fmt.Sprintf("ravendb-certs-%s", node),
// 					Namespace: fixture.Namespace,
// 				},
// 				Type: corev1.SecretTypeOpaque,
// 				Data: map[string][]byte{
// 					"server.pfx": []byte(fmt.Sprintf("fake-pfx-for-%s", node)),
// 				},
// 			}
// 			Expect(fixture.K8sClient.Create(fixture.Ctx, secret)).To(Succeed())
// 		}

// 		instance := fixture.BaseCluster.DeepCopy()
// 		instance.Spec.Mode = "LetsEncrypt"
// 		instance.Spec.Email = "omer.ratsaby@ravendb.net"
// 		instance.Spec.ServerUrl = "https://0.0.0.0:443"
// 		instance.Spec.ServerUrlTcp = "tcp://0.0.0.0:38888"
// 		for i, node := range instance.Spec.Nodes {
// 			node.PublicServerUrl = fmt.Sprintf("https://%s.thegoldenplatypus.development.run:443", node.Tag)
// 			node.PublicServerUrlTcp = fmt.Sprintf("tcp://%s-tcp.thegoldenplatypus.development.run:443", node.Tag)
// 			node.CertsSecretRef = fmt.Sprintf("ravendb-certs-%s", node.Tag)
// 			instance.Spec.Nodes[i] = node
// 		}

// 		By("Creating RavenDBCluster CR")
// 		Expect(fixture.K8sClient.Create(fixture.Ctx, instance)).To(Succeed())

// 		By("Verifying resources and status")
// 		fixture.VerifyResources(instance)
// 		fixture.VerifyClusterStatus(instance, 3)
// 	})
// })
