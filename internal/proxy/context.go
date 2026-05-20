package proxy

import (
	"context"
	"net/http"
	"time"
)

// contextWithTimeout returns a child context with a timeout attached to req.
// If req already carries a deadline that is sooner, it is preserved.
func contextWithTimeout(req *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(timeout)

	// If the incoming context already has a sooner deadline, don't override it.
	if existing, ok := req.Context().Deadline(); ok && existing.Before(deadline) {
		return context.WithCancel(req.Context())
	}

	return context.WithDeadline(req.Context(), deadline)
}
