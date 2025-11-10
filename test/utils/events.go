package testutil

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func OperatorEventsTSVAll(ctx context.Context) (string, error) {
	const script = `
kubectl get events.events.k8s.io -A -o json | jq -r '
  .items
  | map(select(.reportingController=="ravendb-operator"))
  | sort_by(.eventTime // .deprecatedLastTimestamp // .metadata.creationTimestamp)
  | .[]
  | [
      (.eventTime // .deprecatedLastTimestamp // .metadata.creationTimestamp),
      .type, .reason, .note,
      (.regarding.kind + "/" + .regarding.name),
      .reportingInstance
    ] | @tsv'
`
	c := exec.CommandContext(ctx, "sh", "-lc", script)
	var out bytes.Buffer
	c.Stdout, c.Stderr = &out, &out
	err := c.Run()
	return out.String(), err
}

func RequireContainsAll(
	t *testing.T,
	fetch func() (string, error),
	needles []string,
	grace time.Duration,
) string {
	deadline := time.Now().Add(grace)
	var out string
	for {
		var err error
		out, err = fetch()
		require.NoError(t, err, "fetch events")

		missing := make([]string, 0, len(needles))
		for _, n := range needles {
			if !strings.Contains(out, n) {
				missing = append(missing, n)
			}
		}
		if len(missing) == 0 {
			return out
		}
		if time.Now().After(deadline) {
			const max = 6000
			preview := out
			if len(preview) > max {
				preview = preview[:max] + "... (truncated)"
			}
			require.Failf(t, "missing expected substrings",
				"missing:\n  - %s\nin events output (preview):\n%s",
				strings.Join(missing, "\n  - "), preview)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func WaitForEventSubstring(t *testing.T, fetch func() (string, error), substr string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var out string
	for {
		var err error
		out, err = fetch()
		require.NoError(t, err, "fetch events")
		if strings.Contains(out, substr) {
			return out
		}
		if time.Now().After(deadline) {
			require.Failf(t, "event not found in time",
				"missing substring after %s:\n%q\npreview:\n%s",
				timeout, substr, crop(out, 6000))
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func RequireContainsAny(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return
		}
	}
	require.Failf(t, "missing expected substring (any-of)",
		"none of the expected substrings found:\n  - %q", strings.Join(needles, "\n  - "))
}

func RequireNotContainsAny(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			require.Failf(t, "unexpected event present", "found unexpected substring: %q\npreview:\n%s", n, crop(haystack, 6000))
		}
	}
}
func crop(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "... (truncated)"
}

func RequireContainsAnyEventually(
	t *testing.T,
	fetch func() (string, error),
	timeout time.Duration,
	needles ...string,
) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var out string

	for {
		var err error
		out, err = fetch()
		require.NoError(t, err, "fetch events")

		for _, n := range needles {
			if strings.Contains(out, n) {
				return out
			}
		}

		if time.Now().After(deadline) {
			require.Failf(t, "missing expected substring (any-of)",
				"none of the expected substrings found after %s:\n  - %s\npreview:\n%s",
				timeout, strings.Join(needles, "\n  - "), crop(out, 6000))
		}

		time.Sleep(200 * time.Millisecond)
	}
}

func RequireContainsAllEventually(
	t *testing.T,
	fetch func() (string, error),
	expected []string,
	timeout time.Duration,
) string {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var out string

	for {
		var err error
		out, err = fetch()
		require.NoError(t, err, "fetch events")

		missing := make([]string, 0, len(expected))
		for _, n := range expected {
			if !strings.Contains(out, n) {
				missing = append(missing, n)
			}
		}

		if len(missing) == 0 {
			return out
		}

		if time.Now().After(deadline) {
			preview := crop(out, 6000)
			require.Failf(t, "missing expected substrings (all-of)",
				"missing:\n  - %s\nin events output (preview):\n%s",
				strings.Join(missing, "\n  - "), preview)
		}

		time.Sleep(200 * time.Millisecond)
	}
}
