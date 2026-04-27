package actions

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// BackoffWaitWithContext retries cond using exponential backoff, stopping if ctx
// is cancelled. cond receives the context so it can propagate cancellation to
// downstream calls.
func BackoffWaitWithContext(ctx context.Context, steps int, cond func(context.Context) (bool, error)) error {
	return wait.ExponentialBackoffWithContext(ctx, wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   1.1,
		Jitter:   0.1,
		Steps:    steps,
	}, cond)
}
