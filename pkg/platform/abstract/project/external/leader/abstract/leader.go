/*
Copyright 2026 The Nuclio Authors.

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

package abstract

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/nuclio/nuclio/pkg/auth"
	"github.com/nuclio/nuclio/pkg/platform"
	leaderCommon "github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader"
	"github.com/nuclio/nuclio/pkg/platformconfig"
)

// LeaderOps provides default implementations of leader.LeaderOps methods that are
// identical across multiple concrete leaders — e.g. leaders with no async job system,
// or leaders that do not (yet) support the two-phase-commit project sync protocol.
//
// Concrete leader implementations embed this type and override only the methods that
// differ for their specific leader, mirroring the same embedding pattern abstract.Platform
// uses for the kube/local platform.Platform implementations.
type LeaderOps struct{}

// NewLeaderOps creates a new abstract LeaderOps providing the shared default behavior.
func NewLeaderOps() *LeaderOps {
	return &LeaderOps{}
}

// AddAuthSessionHeaders adds the standard authorization header derived from the auth
// session. Every current leader authenticates this way.
func (l *LeaderOps) AddAuthSessionHeaders(headers map[string]string, authSession auth.Session) {
	headers["authorization"] = authSession.CompileAuthorizationHeader()
}

func (l *LeaderOps) ProjectRequestURL(apiAddress string, apiVersion leaderCommon.APIVersion, projectName string) string {
	url := fmt.Sprintf("%s/%s/%s", apiAddress, apiVersion, "projects")
	if projectName != "" {
		url += fmt.Sprintf("/%s", projectName)
	}
	return url
}

// ShouldWaitForCreateCompletion defaults to false: most leaders have no async job
// system to poll after creation succeeds.
func (l *LeaderOps) ShouldWaitForCreateCompletion() bool { return false }

// GetJobStatusRequestCookies defaults to no cookies being required for job status requests.
func (l *LeaderOps) GetJobStatusRequestCookies(_ *platformconfig.Config) []*http.Cookie { return nil }

// GetJobRequestFilter defaults to no filter being applied to job status requests.
func (l *LeaderOps) GetJobRequestFilter(_ *time.Time) string { return "" }

// GetAuthSessionCookie defaults to no session cookie being attached to requests.
func (l *LeaderOps) GetAuthSessionCookie(_ auth.Session) *http.Cookie { return nil }

func (l *LeaderOps) GetDeleteStrategyHeaderName() string {
	return ""
}

// ParseJobStatusResponse defaults to "no job, not terminated": leaders without an
// async job system never have a job to poll.
func (l *LeaderOps) ParseJobStatusResponse(_ context.Context, _ []byte) (leaderCommon.JobResponse, bool) {
	return nil, false
}

// GetJobIdUrl defaults to an empty URL: leaders without an async job system never
// need to poll job status.
func (l *LeaderOps) GetJobIdUrl(_, _ string) string { return "" }

// IsJobCompleted defaults to success: leaders without an async job system never have
// a job to validate.
func (l *LeaderOps) IsJobCompleted(_ context.Context, _ leaderCommon.JobResponse, _ string) error {
	return nil
}

// EvaluateLeaderRequest defaults to an unconditional pass-through: leaders that do not
// (yet) support the two-phase-commit project sync protocol always apply the incoming change.
func (l *LeaderOps) EvaluateLeaderRequest(_ context.Context, _ map[string]string, _ platform.Project) (bool, error) {
	return true, nil
}

// ProjectSync2PCEnabled defaults to false: two-phase-commit support is opt-in per leader.
func (l *LeaderOps) ProjectSync2PCEnabled() bool { return false }
