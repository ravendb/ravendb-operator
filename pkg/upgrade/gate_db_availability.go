package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type dbStatus struct {
	LastStatus string
	LastError  string
}

type nodesTopology struct {
	Members     []map[string]any
	Promotables []map[string]any
	Rehabs      []map[string]any
	Status      map[string]dbStatus
}

type databaseInfo struct {
	Name              string
	Disabled          bool
	ReplicationFactor int
	NodesTopology     nodesTopology
}

type databasesResponse struct {
	Databases []databaseInfo
}

var ignoredErrSnippets = []string{
	"(status: loading)", "not responding", "connection refused",
	"serviceunavailable", "node in rehabilitation",
}

func (hcc *HealthCheckContext) DatabasesOnline(ctx context.Context, excludedTag string) (bool, string, error) {
	baseURL, err := hcc.clusterURL()
	if err != nil {
		return false, "", err
	}

	endpoint, err := join(baseURL, "/databases")
	if err != nil {
		return false, "", err
	}

	code, body, err := hcc.httpGET(ctx, endpoint)
	if err != nil {
		return false, err.Error(), nil
	}
	if code < 200 || code >= 300 {
		return false, fmt.Sprintf("HTTP %d (%s)", code, truncate(body, 200)), nil
	}

	var dr databasesResponse
	if json.Unmarshal([]byte(body), &dr) != nil {
		return false, "invalid /databases response", nil
	}
	if len(dr.Databases) == 0 {
		return true, "no databases", nil
	}

	for _, db := range dr.Databases {
		if db.Disabled || db.ReplicationFactor == 1 {
			continue
		}

		nodes := append(
			append(db.NodesTopology.Members, db.NodesTopology.Promotables...),
			db.NodesTopology.Rehabs...,
		)

		allTags := pluckTags(nodes)

		var okNodes []string
		var firstNonIgnored string

		for _, tag := range allTags {
			if strings.EqualFold(tag, excludedTag) {
				continue
			}

			status := db.NodesTopology.Status[tag]
			if strings.EqualFold(strings.TrimSpace(status.LastStatus), "ok") {
				okNodes = append(okNodes, tag)
				continue
			}

			if isHardLoadError(status.LastError) {
				return false, fmt.Sprintf("db=%s node=%s error=%s", db.Name, tag, summarizeError(status.LastError)), nil
			}

			if status.LastError != "" && !isIgnoredTransient(status.LastError) && firstNonIgnored == "" {
				firstNonIgnored = fmt.Sprintf("db=%s node=%s error=%s", db.Name, tag, summarizeError(status.LastError))
			}
		}

		if len(okNodes) > 0 {
			continue
		}

		if firstNonIgnored != "" {
			return false, firstNonIgnored, nil
		}

		return false, fmt.Sprintf("db=%s reason=no usable member with LastStatus==Ok", db.Name), nil
	}

	return true, "", nil
}

func isHardLoadError(s string) bool {
	return strings.Contains(strings.ToLower(s), "endofstreamexception")
}

func isIgnoredTransient(s string) bool {
	err := strings.ToLower(s)
	for _, sub := range ignoredErrSnippets {
		if strings.Contains(err, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

func pluckTags(arr []map[string]any) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(arr))
	for _, m := range arr {
		if tag, _ := m["NodeTag"].(string); tag != "" {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; !ok {
				seen[tag] = struct{}{}
				out = append(out, tag)
			}
		}
	}
	return out
}
