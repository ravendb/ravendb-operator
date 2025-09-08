package testutil

import (
	"regexp"
	"strings"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var rfc1123Regexp = regexp.MustCompile("[^a-z0-9.-]+")

func SanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = rfc1123Regexp.ReplaceAllString(name, "-")
	return strings.Trim(name, "-")
}

func Key(ns, name string) ctrlclient.ObjectKey {
	return ctrlclient.ObjectKey{Namespace: ns, Name: name}
}
