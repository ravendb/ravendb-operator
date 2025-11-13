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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Timing struct {
	PreMaxWait      time.Duration
	PostMaxWait     time.Duration
	PingInterval    time.Duration
	DBInterval      time.Duration
	GraceAfterReady time.Duration
}

func (u *upgrader) SetTiming(t Timing) { u.timing = t }
func timestampNow() metav1.Time        { return metav1.Now() }

func (u *upgrader) waitNodeAlive(ctx context.Context, c *ravendbv1alpha1.RavenDBCluster, hcc *HealthCheckContext, phase GatePhase, tag string) error {
	return u.wait(ctx, c, phase, GateNodeAlive, tag, u.timing.PingInterval, func() (bool, string, error) {
		return hcc.NodeAlive(ctx, tag)
	})
}

func (u *upgrader) waitConnectivity(ctx context.Context, c *ravendbv1alpha1.RavenDBCluster, hcc *HealthCheckContext, phase GatePhase, tag string) error {
	return u.wait(ctx, c, phase, GateClusterConnectivity, tag, u.timing.PingInterval, func() (bool, string, error) {
		return hcc.ClusterConnectivity(ctx)
	})
}

func (u *upgrader) waitDB(ctx context.Context, c *ravendbv1alpha1.RavenDBCluster, hcc *HealthCheckContext, phase GatePhase, excluded string, tag string) error {
	return u.wait(ctx, c, phase, GateDatabasesOnline, tag, u.timing.DBInterval, func() (bool, string, error) {
		return hcc.DatabasesOnline(ctx, excluded)
	})
}

func (u *upgrader) wait(
	ctx context.Context,
	c *ravendbv1alpha1.RavenDBCluster,
	phase GatePhase,
	kind GateKind,
	tag string,
	interval time.Duration,
	fn func() (bool, string, error),
) error {

	// choose how long we are allowed to wait
	maxWait := u.maxWaitFor(phase)
	maxSleep := 15 * time.Second

	// announce we started current gate
	if u.emit != nil {
		u.emit(c, GateStart, phase, kind, tag, "")
	}

	start := time.Now()
	sleep := interval
	attempt := 0
	lastInfo := ""

	for {
		// if something canceled the context, stop right away
		if err := ctx.Err(); err != nil {
			msg := err.Error()
			if msg == "" {
				msg = "timeout"
			}
			if u.emit != nil {
				u.emit(c, GateTimeout, phase, kind, tag, msg)
			}
			return &GateError{Phase: phase, Kind: kind, Tag: tag, Info: msg}
		}

		// actual gate check
		ok, info, err := fn()
		if err != nil {
			// hard error from the check -> fail immediately
			if u.emit != nil {
				u.emit(c, GateBlock, phase, kind, tag, err.Error())
			}
			return &GateError{Phase: phase, Kind: kind, Tag: tag, Info: err.Error()}
		}

		if ok { // success
			if u.emit != nil {
				u.emit(c, GatePass, phase, kind, tag, "")
			}
			return nil
		}

		// not ok yet -> store info and announce block
		lastInfo = info
		attempt++
		if u.emit != nil {
			u.emit(c, GateBlock, phase, kind, tag,
				fmt.Sprintf("retry in %s (attempt %d): %s", sleep, attempt, summarizeError(lastInfo)))
		}

		// check if we did we run out of time
		if time.Since(start) >= maxWait {
			msg := lastInfo
			if msg == "" {
				msg = "timeout"
			} else {
				msg = msg + " (timeout)"
			}
			if u.emit != nil {
				u.emit(c, GateTimeout, phase, kind, tag, msg)
			}
			return &GateError{Phase: phase, Kind: kind, Tag: tag, Info: msg}
		}

		// sleep a bit and try again
		time.Sleep(sleep)
		sleep = sleep * 2
		if sleep > maxSleep {
			sleep = maxSleep
		}
	}
}

func (u *upgrader) maxWaitFor(phase GatePhase) time.Duration {
	if phase == GatePreStep {
		return u.timing.PreMaxWait
	}
	return u.timing.PostMaxWait
}
