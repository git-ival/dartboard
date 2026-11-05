package actions

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

var errCheckpointFailed = errors.New("cluster checkpoint failed")

type clusterPacer struct {
	next     time.Time
	interval time.Duration
	jitter   float64
	mu       sync.Mutex
}

func newClusterPacer(interval time.Duration, jitter float64) (*clusterPacer, error) {
	if interval < 0 {
		return nil, fmt.Errorf("cluster_start_interval must not be negative")
	}

	if math.IsNaN(jitter) || math.IsInf(jitter, 0) || jitter < 0 || jitter > 1 {
		return nil, fmt.Errorf("cluster_start_jitter must be between zero and one")
	}

	return &clusterPacer{interval: interval, jitter: jitter}, nil
}

func (pacer *clusterPacer) wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if pacer.interval == 0 {
		return nil
	}

	pacer.mu.Lock()

	start := time.Now()
	if pacer.next.After(start) {
		start = pacer.next
	}

	interval := pacer.interval
	if pacer.jitter > 0 {
		interval = kwait.Jitter(interval, pacer.jitter)
	}

	pacer.next = start.Add(interval)
	pacer.mu.Unlock()

	timer := time.NewTimer(time.Until(start))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

func runClusterJobs[Job any](ctx context.Context, jobs []Job, concurrency int,
	name func(Job) string, run func(context.Context, Job) error,
) error {
	if concurrency <= 0 {
		return fmt.Errorf("cluster_batch_size must be greater than zero")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var group errgroup.Group
	group.SetLimit(concurrency)

	jobErrors := make([]error, len(jobs))
	seen := make(map[string]bool, len(jobs))

	for index, job := range jobs {
		if ctx.Err() != nil {
			break
		}

		jobName := name(job)
		if seen[jobName] {
			jobErrors[index] = fmt.Errorf("duplicate cluster identity %q", jobName)
			continue
		}

		seen[jobName] = true

		group.Go(func() error {
			if ctx.Err() != nil {
				return nil
			}

			if err := run(ctx, job); err != nil {
				jobErrors[index] = fmt.Errorf("cluster %s: %w", jobName, err)
				if errors.Is(err, errCheckpointFailed) {
					cancel()
				}
			}

			return nil
		})
	}

	groupErr := group.Wait()

	return errors.Join(errors.Join(jobErrors...), groupErr, ctx.Err())
}
