package actions

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/rancher/shepherd/clients/rancher"
	shepherddefaults "github.com/rancher/shepherd/extensions/defaults"
	"github.com/sirupsen/logrus"
)

// ClientFactory creates a new Rancher client for each goroutine, ensuring
// thread safety since *rancher.Client is not safe for concurrent use.
type ClientFactory func() (*rancher.Client, error)

// ClusterJob defines the interface for any cluster operation that can be
// executed concurrently. Each job type (import, provision, register) implements
// this interface, enabling polymorphic batch processing.
type ClusterJob interface {
	// Name returns the unique identifier for this cluster job
	Name() string
	// Execute performs the cluster operation, returning an error if it fails
	Execute(ctx context.Context, client *rancher.Client, config *rancher.Config) error
	// IsCompleted checks if this job has already been completed (for skip logic)
	IsCompleted(status *ClusterStatus) bool
	// MarkCompleted updates the ClusterStatus to reflect job completion
	MarkCompleted(status *ClusterStatus)
	// FinalStage returns the stage to set when the job completes
	FinalStage() Stage
}

// BatchRunner processes cluster jobs concurrently with state tracking.
// It uses a WaitGroup for goroutine lifecycle management, collecting all
// errors rather than cancelling on the first failure. Concurrency is
// limited with a semaphore.
type BatchRunner struct {
	statuses      map[string]*ClusterStatus
	clientFactory ClientFactory
	config        *rancher.Config
	statePath     string
	maxWorkers    int
	stateMu       sync.Mutex
}

// NewBatchRunner creates a runner with sensible defaults.
// clientFactory is called once per goroutine to produce an isolated *rancher.Client.
func NewBatchRunner(statePath string, statuses map[string]*ClusterStatus, clientFactory ClientFactory, config *rancher.Config) *BatchRunner {
	return &BatchRunner{
		maxWorkers:    runtime.GOMAXPROCS(0) * 2,
		statePath:     statePath,
		statuses:      statuses,
		clientFactory: clientFactory,
		config:        config,
	}
}

// Run executes all jobs concurrently, collecting errors from all jobs before
// returning. A single job failure does not cancel sibling jobs. Concurrency
// is limited to maxWorkers via a semaphore.
func (br *BatchRunner) Run(ctx context.Context, jobs []ClusterJob) error {
	if len(jobs) == 0 {
		return nil
	}

	// Semaphore pattern for limiting concurrency
	sem := make(chan struct{}, br.maxWorkers)

	var (
		wg           sync.WaitGroup
		errMu        sync.Mutex
		errs         []error
		skippedMu    sync.Mutex
		skippedCount int
	)

	for _, job := range jobs {
		job := job // capture for goroutine

		wg.Add(1)

		go func() {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				errMu.Lock()

				errs = append(errs, fmt.Errorf("job %s cancelled: %w", job.Name(), ctx.Err()))
				errMu.Unlock()

				return
			}

			// Initialize status and check if already completed
			status := br.getOrCreateStatus(job.Name())

			if job.IsCompleted(status) {
				logrus.Infof("Cluster %s already completed, skipping...", job.Name())
				skippedMu.Lock()
				skippedCount++
				skippedMu.Unlock()

				return
			}

			// Update state to show job started
			br.updateState(job.Name(), StageNew)

			// Create a per-goroutine client to avoid data races
			client, err := br.clientFactory()
			if err != nil {
				errMu.Lock()

				errs = append(errs, fmt.Errorf("job %s: failed to create client: %w", job.Name(), err))
				errMu.Unlock()

				return
			}

			// Execute the job
			if err := job.Execute(ctx, client, br.config); err != nil {
				errMu.Lock()

				errs = append(errs, fmt.Errorf("job %s failed: %w", job.Name(), err))
				errMu.Unlock()

				return
			}

			// Mark completed and persist
			br.stateMu.Lock()
			job.MarkCompleted(status)
			status.Stage = job.FinalStage()

			if err := SaveClusterState(br.statePath, br.statuses); err != nil {
				logrus.Errorf("failed to save state for %s: %v", job.Name(), err)
			}
			br.stateMu.Unlock()

			logrus.Infof("Cluster %s completed successfully", job.Name())
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		return joinErrors(errs)
	}

	// Batch sleep logic: if fewer than half were skipped, sleep before next batch.
	// This gives Rancher time to stabilize between bursts of cluster operations.
	if skippedCount < len(jobs)/2 {
		logrus.Infof("Batch done: %d/%d skipped; sleeping before next batch.", skippedCount, len(jobs))
		time.Sleep(shepherddefaults.TwoMinuteTimeout)
	} else {
		logrus.Infof("Batch done: %d/%d skipped; continuing without sleep.", skippedCount, len(jobs))
	}

	return nil
}

// getOrCreateStatus retrieves or creates a ClusterStatus entry (thread-safe)
func (br *BatchRunner) getOrCreateStatus(name string) *ClusterStatus {
	br.stateMu.Lock()
	defer br.stateMu.Unlock()

	return FindOrCreateStatusByName(br.statuses, name)
}

// updateState updates and persists cluster stage (thread-safe)
func (br *BatchRunner) updateState(name string, stage Stage) {
	br.stateMu.Lock()
	defer br.stateMu.Unlock()

	cs := br.statuses[name]
	if cs == nil {
		return
	}

	cs.Stage = stage

	if err := SaveClusterState(br.statePath, br.statuses); err != nil {
		logrus.Errorf("failed to save state for %s:%s: %v", name, stage, err)
	}
}

// joinErrors combines multiple errors into a single error message.
func joinErrors(errs []error) error {
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}

	return fmt.Errorf("%d job(s) failed:\n  - %s", len(errs), joinStrings(msgs, "\n  - "))
}

func joinStrings(ss []string, sep string) string {
	result := ""

	for i, s := range ss {
		if i > 0 {
			result += sep
		}

		result += s
	}

	return result
}
