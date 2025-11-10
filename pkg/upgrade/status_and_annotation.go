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
	"fmt"
	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"ravendb-operator/pkg/common"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
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

func ReadTimingFromAnnotations(c *ravendbv1alpha1.RavenDBCluster, def Timing) Timing {
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

func failedStatus(tag, msg, desired string) ravendbv1alpha1.RavenDBNodeStatus {
	return ravendbv1alpha1.RavenDBNodeStatus{
		Tag:                tag,
		Status:             ravendbv1alpha1.NodeStatusFailed,
		LastAttemptedImage: desired,
		LastError:          msg,
		LastAttemptTime:    timestampNow(),
	}
}

func successStatus(tag, desired string) ravendbv1alpha1.RavenDBNodeStatus {
	return ravendbv1alpha1.RavenDBNodeStatus{
		Tag:                tag,
		Status:             ravendbv1alpha1.NodeStatusCreated,
		LastAttemptedImage: desired,
		LastAttemptTime:    timestampNow(),
	}
}

func statefulSetName(tag string) string {
	return fmt.Sprintf("%s%s", common.Prefix, strings.ToLower(tag))
}

// toggles the per-node STS annotation so the actor switches the image
func (u *upgrader) setUpgradeAnnotation(ctx context.Context, kc client.Client, c *ravendbv1alpha1.RavenDBCluster, tag, value string) error {
	stsName := statefulSetName(tag)
	var sts appsv1.StatefulSet

	err := kc.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: stsName}, &sts)
	if err != nil {
		if kerrors.IsNotFound(err) {
			// nothing to do - sts is gone
			return nil
		}
		return err
	}

	old := sts.DeepCopy()
	if sts.Annotations == nil {
		sts.Annotations = map[string]string{}
	}

	if value == "" {
		delete(sts.Annotations, common.UpgradeImageAnnotation)
	} else {
		sts.Annotations[common.UpgradeImageAnnotation] = value
	}

	return kc.Patch(ctx, &sts, client.MergeFrom(old))
}

func (u *upgrader) hasUpgradeAnnotation(ctx context.Context, kc client.Client, c *ravendbv1alpha1.RavenDBCluster, tag string) (bool, error) {
	var sts appsv1.StatefulSet
	if err := kc.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: statefulSetName(tag)}, &sts); err != nil {
		return false, err
	}
	if sts.Annotations == nil {
		return false, nil
	}
	_, ok := sts.Annotations[common.UpgradeImageAnnotation]
	return ok, nil
}
