/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package upgrade

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/common"

	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func DefaultTiming() Timing {
	return Timing{
		PreMaxWait:      5 * time.Minute,  // covers 2m node_alive / 2m connectivity / 5m DB
		PostMaxWait:     12 * time.Minute, // typical small-cluster post checks
		PingInterval:    5 * time.Second,
		DBInterval:      10 * time.Second,
		GraceAfterReady: 10 * time.Second,
	}
}

func ReadTimingFromAnnotations(c *ravendbv1.RavenDBCluster, def Timing) Timing {
	anns := c.GetAnnotations()
	if anns == nil {
		return def
	}
	parse := func(key string, dst *time.Duration) {
		if v, ok := anns[key]; ok && strings.TrimSpace(v) != "" {
			if d, err := time.ParseDuration(v); err == nil && d > 0 {
				*dst = d
			}
		}
	}
	parse(common.UpgradePreWaitAnnotation, &def.PreMaxWait)
	parse(common.UpgradePostWaitAnnotation, &def.PostMaxWait)
	parse(common.UpgradePingIntervalAnnotation, &def.PingInterval)
	parse(common.UpgradeDBIntervalAnnotation, &def.DBInterval)
	return def
}

func failedStatus(tag, msg, desired string) ravendbv1.RavenDBNodeStatus {
	return ravendbv1.RavenDBNodeStatus{
		Tag:                tag,
		Status:             ravendbv1.NodeStatusFailed,
		LastAttemptedImage: desired,
		LastError:          msg,
		LastAttemptTime:    timestampNow(),
	}
}

func successStatus(tag, desired string) ravendbv1.RavenDBNodeStatus {
	return ravendbv1.RavenDBNodeStatus{
		Tag:                tag,
		Status:             ravendbv1.NodeStatusCreated,
		LastAttemptedImage: desired,
		LastAttemptTime:    timestampNow(),
	}
}

func statefulSetName(tag string) string {
	return common.NodeResourceName(tag)
}

// setUpgradeAnnotation toggles the durable per-node rollout marker with a
// direct merge patch. It deliberately performs no cached read: both setting
// the marker before apply and clearing it after a no-gate recovery rollout are
// transaction boundaries, so neither may depend on cache convergence.
func (u *upgrader) setUpgradeAnnotation(ctx context.Context, kc client.Client, c *ravendbv1.RavenDBCluster, tag, value string) error {
	var annotationValue any = value
	if value == "" {
		annotationValue = nil
	}

	data, err := json.Marshal(map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				common.UpgradeImageAnnotation: annotationValue,
			},
		},
	})
	if err != nil {
		return err
	}

	sts := &appsv1.StatefulSet{}
	sts.Namespace = c.Namespace
	sts.Name = statefulSetName(tag)
	err = kc.Patch(ctx, sts, client.RawPatch(types.MergePatchType, data))
	if kerrors.IsNotFound(err) {
		// A missing StatefulSet is handled by node selection/creation.
		return nil
	}
	return err
}
