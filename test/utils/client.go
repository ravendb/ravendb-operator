package testutil

import (
	"testing"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func K8sClient(t *testing.T) ctrlclient.Client {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, ravendbv1alpha1.AddToScheme(s))

	cfg := kubeRestConfig(t)
	cli, err := ctrlclient.New(cfg, ctrlclient.Options{Scheme: s})
	require.NoError(t, err)
	return cli
}

func kubeRestConfig(t *testing.T) *rest.Config {
	t.Helper()
	ldr := clientcmd.NewDefaultClientConfigLoadingRules()
	ovr := &clientcmd.ConfigOverrides{}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(ldr, ovr).ClientConfig()
	require.NoError(t, err)
	return cfg
}
