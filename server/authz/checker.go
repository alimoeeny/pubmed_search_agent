// Package authz provides pluggable authorization for the HTTP server.
package authz

import (
	"context"
	"net/http"

	"github.com/alimoeeny/pubmed_search_agent/user"
)

// AuthorizationChecker decides whether a request is allowed to proceed.
// Implementations should return an *AuthzError to send a specific HTTP status;
// any other error produces a 500.
type AuthorizationChecker interface {
	Check(ctx context.Context, profile user.UserProfile) error
}

// AuthzError carries the HTTP status code and message sent to the client.
type AuthzError struct {
	Status  int
	Message string
}

func (e *AuthzError) Error() string { return e.Message }

// NoOpChecker always passes — used in local dev and tests.
type NoOpChecker struct{}

func (NoOpChecker) Check(_ context.Context, _ user.UserProfile) error { return nil }

// PlanChecker enforces per-plan request limits.
// Limits maps each plan to a max monthly request count; use -1 for unlimited.
type PlanChecker struct {
	Limits map[user.SubscriptionPlan]int
}

// DefaultPlanLimits provides sensible defaults for each subscription tier.
var DefaultPlanLimits = map[user.SubscriptionPlan]int{
	user.PlanFree: 10,
	user.PlanPro:  100,
	user.PlanMax:  -1,
}

// Check implements AuthorizationChecker.
func (c *PlanChecker) Check(_ context.Context, profile user.UserProfile) error {
	if !profile.Enabled {
		return &AuthzError{Status: http.StatusForbidden, Message: "account disabled"}
	}
	limit, ok := c.Limits[profile.Plan]
	if !ok || limit < 0 {
		return nil
	}
	// TODO: count monthly queries from DB and enforce limit.
	return nil
}
