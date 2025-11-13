package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
)

func (hcc *HealthCheckContext) ClusterConnectivity(ctx context.Context) (bool, string, error) {
	baseURL, err := hcc.clusterURL()
	if err != nil {
		return false, "", err
	}

	endpoint, err := join(baseURL, "/admin/debug/node/ping")
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

	var pr pingResponse
	if err := json.Unmarshal([]byte(body), &pr); err != nil {
		return false, "invalid ping response: bad JSON", nil
	}
	if len(pr.Result) == 0 {
		return false, "invalid ping response: empty result", nil
	}

	for _, it := range pr.Result {
		setupErr := summarizeError(it.SetupAlive.Error)
		tcpErr := summarizeError(it.TcpInfo.Error)
		if setupErr != "" || tcpErr != "" {
			return false, fmt.Sprintf("peer=%s setup_err=%q tcp_err=%q",
				it.Url, setupErr, tcpErr), nil
		}
	}
	return true, "", nil
}
