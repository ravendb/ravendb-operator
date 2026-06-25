package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestBuildPodSecurityContextSetsRavenDBFsGroup(t *testing.T) {
	sc := buildPodSecurityContext()

	require.NotNil(t, sc)
	require.NotNil(t, sc.FSGroup)
	require.EqualValues(t, 999, *sc.FSGroup)
	require.NotNil(t, sc.FSGroupChangePolicy)
	require.Equal(t, corev1.FSGroupChangeOnRootMismatch, *sc.FSGroupChangePolicy)
	require.Contains(t, sc.Sysctls, corev1.Sysctl{
		Name:  "net.ipv4.ip_unprivileged_port_start",
		Value: "0",
	})
}
