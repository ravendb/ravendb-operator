/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package common

import "strings"

// NodeResourceName returns the per-node Kubernetes object name for a RavenDB
// node tag. The tag is lowercased because Kubernetes object names must satisfy
// RFC 1035 / 1123 (lowercase alphanumeric); the CR-level spec.nodes[].tag is
// preserved as-is — RavenDB convention keeps it uppercase ("A", "B", …).
//
// Used by Service, StatefulSet, Ingress backend refs, and per-node DNS endpoints
// so all of them agree on a single canonical name.
func NodeResourceName(tag string) string {
	return Prefix + strings.ToLower(tag)
}

// NodePodName returns the name of the single Pod backing a node. Each node runs
// its own StatefulSet at NumOfReplicas (1), and a StatefulSet names its Pods
// "<set>-<ordinal>", so ordinal 0 is the entire node.
func NodePodName(tag string) string {
	return NodeResourceName(tag) + "-0"
}
