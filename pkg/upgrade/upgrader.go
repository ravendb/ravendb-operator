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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	appsv1 "k8s.io/api/apps/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ravendbv1alpha1 "ravendb-operator/api/v1alpha1"
	"ravendb-operator/pkg/common"
)

type Upgrader interface {
	Run(ctx context.Context, cluster *ravendbv1alpha1.RavenDBCluster, kc client.Client, applyNode ApplyNodeFn) ([]ravendbv1alpha1.RavenDBNodeStatus, error)
	SetEmitter(GateEmitter)
	SetTiming(Timing)
}

type upgrader struct {
	buildGates func(ctx context.Context, kc client.Client, c *ravendbv1alpha1.RavenDBCluster) (*HealthCheckContext, error)
	timing     Timing
	emit       GateEmitter
}

type GateState string

const (
	GateStart   GateState = "start"
	GatePass    GateState = "pass"
	GateBlock   GateState = "block"
	GateTimeout GateState = "timeout"
)

type ApplyNodeFn func(node ravendbv1alpha1.RavenDBNode) error
type GateEmitter func(cluster *ravendbv1alpha1.RavenDBCluster, state GateState, phase GatePhase, kind GateKind, tag, info string)

func (u *upgrader) SetEmitter(e GateEmitter) { u.emit = e }
func normalizeTag(t string) string           { return strings.ToUpper(strings.TrimSpace(t)) }

func NewUpgrader(t Timing) Upgrader {
	if t.PreMaxWait == 0 {
		t.PreMaxWait = 5 * time.Minute
	}
	if t.PostMaxWait == 0 {
		t.PostMaxWait = 15 * time.Minute
	}
	if t.PingInterval == 0 {
		t.PingInterval = 5 * time.Second
	}
	if t.DBInterval == 0 {
		t.DBInterval = 10 * time.Second
	}
	if t.GraceAfterReady == 0 {
		t.GraceAfterReady = 10 * time.Second
	}

	return &upgrader{
		buildGates: buildGatesDefault,
		timing:     t,
	}
}

func buildGatesDefault(ctx context.Context, kc client.Client, c *ravendbv1alpha1.RavenDBCluster) (*HealthCheckContext, error) {
	httpc, err := BuildHTTPSClientFromCluster(ctx, kc, c)
	if err != nil {
		return nil, err
	}
	return NewChecks(httpc, c), nil
}

// Run() performs exactly one "upgrade tick".
// High-level steps:
//  1. build gates + http client.
//  2. figure out which single node we should work on now.
//  3. if a node was chosen:
//     a) if upgrading, run pre-checks and mark it as upgrading with an annotation.
//     b) call applyNode(node) to mutate its image.
//     c) if upgraded, run post-checks. On failure, mark node status as Failed.
//  4. Return statuses for all nodes.
func (u *upgrader) Run(
	ctx context.Context,
	cluster *ravendbv1alpha1.RavenDBCluster,
	kc client.Client,
	applyNode ApplyNodeFn,
) ([]ravendbv1alpha1.RavenDBNodeStatus, error) {

	// 1) build gates (HTTP client to cluster for checks)
	gates, err := u.buildGates(ctx, kc, cluster)
	if err != nil {
		return nil, err
	}

	desiredImg := desiredNodeImage(cluster)
	prev := buildPrevStatusMap(cluster.Status)

	// 2) decide which node to work on in this tick
	selectedTag, err := u.pickSelectedTag(ctx, kc, cluster, desiredImg)
	if err != nil {
		// on error, fall back to returning current statuses
		out := make([]ravendbv1alpha1.RavenDBNodeStatus, 0, len(cluster.Spec.Nodes))
		for _, n := range cluster.Spec.Nodes {
			out = append(out, statusOrCreated(prev, n.Tag))
		}
		return out, err
	}

	// if nothing to do, just return existing statuses
	if selectedTag == "" {
		out := make([]ravendbv1alpha1.RavenDBNodeStatus, 0, len(cluster.Spec.Nodes))
		for _, n := range cluster.Spec.Nodes {
			out = append(out, statusOrCreated(prev, n.Tag))
		}
		return out, nil
	}

	// 3) iterate all nodes - only mutate the chosen one, keep the rest unchanged
	statuses := make([]ravendbv1alpha1.RavenDBNodeStatus, 0, len(cluster.Spec.Nodes))

	for _, node := range cluster.Spec.Nodes {
		// untouched nodes
		if !strings.EqualFold(node.Tag, selectedTag) {
			statuses = append(statuses, statusOrCreated(prev, node.Tag))
			continue
		}

		// we operate on this node
		sts, stsExists, getErr := u.loadSTSByNodeTag(ctx, kc, cluster, node.Tag)
		if getErr != nil {
			// mark upgrade as failed
			statuses = append(statuses, ravendbv1alpha1.RavenDBNodeStatus{Tag: node.Tag, Status: ravendbv1alpha1.NodeStatusFailed})
			return statuses, getErr
		}

		currentImg := ""
		if sts != nil {
			currentImg = currentStsImage(sts)
		}
		marked, _ := u.hasUpgradeAnnotation(ctx, kc, cluster, node.Tag)
		upgrading := isUpgrading(stsExists, desiredImg, currentImg, marked)

		// BEFORE: if upgrading and not already marked, run gates + set annotations
		if upgrading && !marked {
			if err := u.preNode(ctx, cluster, gates, node.Tag); err != nil {
				statuses = append(statuses, failedStatus(node.Tag, err.Error(), desiredImg))
				return statuses, fmt.Errorf("pre-node gates failed for %s: %w", node.Tag, err)
			}

			// mark upgrade intent with target image
			if err := u.setUpgradeAnnotation(ctx, kc, cluster, node.Tag, desiredImg); err != nil {
				statuses = append(statuses, failedStatus(node.Tag, "set upgrade annotation: "+err.Error(), desiredImg))
				_ = u.setUpgradeAnnotation(ctx, kc, cluster, node.Tag, "")
				return statuses, err
			}
		}

		// MUTATE
		if err := applyNode(node); err != nil {
			statuses = append(statuses, failedStatus(node.Tag, err.Error(), desiredImg))
			if upgrading {
				_ = u.setUpgradeAnnotation(ctx, kc, cluster, node.Tag, "")
			}
			return statuses, fmt.Errorf("apply node %s failed: %w", node.Tag, err)
		}

		// AFTER: only for real upgrades (not first creation)
		if upgrading {
			if err := u.postNode(ctx, cluster, gates, node.Tag); err != nil {
				// just mark failed and clear annotation
				statuses = append(statuses, failedStatus(
					node.Tag,
					err.Error(),
					desiredImg,
				))
				_ = u.setUpgradeAnnotation(ctx, kc, cluster, node.Tag, "")
				return statuses, fmt.Errorf("post-node gates failed for %s: %w", node.Tag, err)
			}

			// success so cleanup annotation
			_ = u.setUpgradeAnnotation(ctx, kc, cluster, node.Tag, "")
			statuses = append(statuses, successStatus(node.Tag, desiredImg))
			continue
		}

		statuses = append(statuses, statusOrCreated(prev, node.Tag))
	}

	// 4) keep order like Spec.Nodes
	byUpper := make(map[string]ravendbv1alpha1.RavenDBNodeStatus, len(statuses))
	for _, s := range statuses {
		byUpper[normalizeTag(s.Tag)] = s
	}
	ordered := make([]ravendbv1alpha1.RavenDBNodeStatus, 0, len(cluster.Spec.Nodes))
	for _, n := range cluster.Spec.Nodes {
		ordered = append(ordered, byUpper[normalizeTag(n.Tag)])
	}
	return ordered, nil
}

// looks for a node which StatefulSet has the "upgrade image" annotation.
// if found, we return that tag to continue the in-progress upgrade.
func (u *upgrader) findInFlightTag(ctx context.Context, kc client.Client, c *ravendbv1alpha1.RavenDBCluster) (string, error) {
	for _, n := range c.Spec.Nodes {
		var sts appsv1.StatefulSet
		err := kc.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: statefulSetName(n.Tag)}, &sts)
		if err == nil {
			if sts.Annotations != nil {
				if _, ok := sts.Annotations[common.UpgradeImageAnnotation]; ok {
					return n.Tag, nil
				}
			}
		}
	}
	return "", nil
}

func (u *upgrader) preNode(ctx context.Context, c *ravendbv1alpha1.RavenDBCluster, hcc *HealthCheckContext, tag string) error {
	if err := u.waitNodeAlive(ctx, c, hcc, GatePreStep, tag); err != nil {
		return err
	}
	if err := u.waitConnectivity(ctx, c, hcc, GatePreStep, tag); err != nil {
		return err
	}
	if err := u.waitDB(ctx, c, hcc, GatePreStep, tag, tag); err != nil {
		return err
	}
	return nil
}

// postNode runs checks AFTER we mutate the node (alive, connectivity, DBs full cluster).
func (u *upgrader) postNode(ctx context.Context, c *ravendbv1alpha1.RavenDBCluster, hcc *HealthCheckContext, tag string) error {
	if err := u.waitNodeAlive(ctx, c, hcc, GatePostStep, tag); err != nil {
		return err
	}

	// grace period so the node finishes bootstrapping before the gates.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(u.timing.GraceAfterReady):
	}

	if err := u.waitConnectivity(ctx, c, hcc, GatePostStep, tag); err != nil {
		return err
	}
	if err := u.waitDB(ctx, c, hcc, GatePostStep, "", tag); err != nil {
		return err
	}
	return nil
}

func desiredNodeImage(c *ravendbv1alpha1.RavenDBCluster) string {
	return c.Spec.Image
}

func currentStsImage(sts *appsv1.StatefulSet) string {
	if len(sts.Spec.Template.Spec.Containers) == 0 {
		return ""
	}
	return sts.Spec.Template.Spec.Containers[0].Image
}

// builds map "tag":: last known status so we keep untouched nodes as is
func buildPrevStatusMap(st ravendbv1alpha1.RavenDBClusterStatus) map[string]ravendbv1alpha1.RavenDBNodeStatus {
	out := make(map[string]ravendbv1alpha1.RavenDBNodeStatus, len(st.Nodes))
	for _, s := range st.Nodes {
		out[normalizeTag(s.Tag)] = s
	}
	return out
}

func isUpgrading(stsExists bool, desiredImg, currentImg string, marked bool) bool {
	if !stsExists {
		return false
	}
	if desiredImg == "" || currentImg == "" {
		return false
	}
	if marked {
		return true
	}
	return desiredImg != currentImg
}

// returns either previous status or default Created
func statusOrCreated(prev map[string]ravendbv1alpha1.RavenDBNodeStatus, nodeTag string) ravendbv1alpha1.RavenDBNodeStatus {
	if prestatus, ok := prev[normalizeTag(nodeTag)]; ok {
		return prestatus
	}
	return ravendbv1alpha1.RavenDBNodeStatus{Tag: nodeTag, Status: ravendbv1alpha1.NodeStatusCreated}
}

func (u *upgrader) pickSelectedTag(ctx context.Context, kc client.Client, c *ravendbv1alpha1.RavenDBCluster, desiredImg string) (string, error) {
	// first we will try find those in the middle of an upgrade
	if t, _ := u.findInFlightTag(ctx, kc, c); strings.TrimSpace(t) != "" {
		return normalizeTag(t), nil
	}

	// if no in-flight upgrade is found, we look for the first node with no sts
	for _, n := range c.Spec.Nodes {
		name := statefulSetName(n.Tag)
		var sts appsv1.StatefulSet
		if err := kc.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: name}, &sts); kerrors.IsNotFound(err) {
			return normalizeTag(n.Tag), nil
		}
	}

	// lastly the first one with image mismatch
	for _, n := range c.Spec.Nodes {
		name := statefulSetName(n.Tag)
		var sts appsv1.StatefulSet
		if err := kc.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: name}, &sts); err == nil {
			cur := currentStsImage(&sts)
			if cur != "" && desiredImg != "" && cur != desiredImg {
				return normalizeTag(n.Tag), nil
			}
		}
	}

	return "", nil
}

func (u *upgrader) loadSTSByNodeTag(ctx context.Context, kc client.Client, c *ravendbv1alpha1.RavenDBCluster, tag string) (*appsv1.StatefulSet, bool, error) {
	var sts appsv1.StatefulSet
	err := kc.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: statefulSetName(tag)}, &sts)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &sts, true, nil
}

func NewGateEventEmitter(kc client.Client, rec record.EventRecorder) GateEmitter {
	return func(
		c *ravendbv1alpha1.RavenDBCluster,
		state GateState,
		phase GatePhase,
		kind GateKind,
		tag, info string,
	) {
		if rec == nil || c == nil {
			return
		}

		eventType := corev1.EventTypeNormal
		if state != GatePass && state != GateStart {
			eventType = corev1.EventTypeWarning
		}

		t := strings.ToUpper(strings.TrimSpace(tag))
		if t == "" {
			t = "-"
		}
		action := fmt.Sprintf("%s/%s", string(phase), string(kind))
		var msg string
		switch state {
		case GateStart:
			msg = fmt.Sprintf("node %s - %s started", t, action)
		case GatePass:
			msg = fmt.Sprintf("node %s - %s passed", t, action)
		case GateBlock:
			msg = fmt.Sprintf("node %s - %s blocked: %s", t, action, info)
		case GateTimeout:
			msg = fmt.Sprintf("node %s - %s timeout: %s", t, action, info)
		}
		reason := fmt.Sprintf("RollingUpgrade_node_%s_%s_%s_%s", phase, kind, state, t)

		if len(reason) > 64 { // keep reason short to avoid folding
			reason = reason[:64]
		}

		rec.Eventf(c, eventType, reason, "%s", msg)

		if tag = strings.TrimSpace(tag); tag != "" {
			stsName := fmt.Sprintf("%s%s", common.Prefix, strings.ToLower(tag))
			var sts appsv1.StatefulSet
			//ignore errors (e.g., during initial creation when STS may not exist yet)
			if err := kc.Get(
				context.Background(),
				client.ObjectKey{Namespace: c.Namespace, Name: stsName},
				&sts,
			); err == nil {
				rec.Eventf(&sts, eventType, reason, "%s", msg)
			}
		}

		// avoid events from sharing identical timestamps which can cause the K8s event aggregator to fold them.
		time.Sleep(200 * time.Millisecond)
	}
}
