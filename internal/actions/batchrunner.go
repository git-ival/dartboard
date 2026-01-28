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
	"golang.org/x/sync/errgroup"
)

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
// It uses the errgroup pattern for clean error handling and context cancellation,
// with a semaphore for limiting concurrency.
type BatchRunner struct {
	maxWorkers int
	statePath  string
	stateMu    sync.Mutex
	statuses   map[string]*ClusterStatus
}

// NewBatchRunner creates a runner with sensible defaults
func NewBatchRunner(statePath string, statuses map[string]*ClusterStatus) *BatchRunner {
	return &BatchRunner{
		maxWorkers: runtime.GOMAXPROCS(0) * 2,
		statePath:  statePath,
		statuses:   statuses,
	}
}

// Run executes jobs concurrently using errgroup for clean error handling.
// It limits concurrency using a semaphore pattern and tracks job state
// for resumability.
func (br *BatchRunner) Run(ctx context.Context, jobs []ClusterJob, client *rancher.Client, config *rancher.Config) error {
	if len(jobs) == 0 {
		return nil
	}

	// Semaphore pattern for limiting concurrency
	sem := make(chan struct{}, br.maxWorkers)

	g, ctx := errgroup.WithContext(ctx)

	skippedCount := 0
	var skippedMu sync.Mutex

	for _, job := range jobs {
		job := job // capture for goroutine

		g.Go(func() error {
			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return ctx.Err()
			}

			// Initialize status and check if already completed
			status := br.getOrCreateStatus(job.Name())

			if job.IsCompleted(status) {
				logrus.Infof("Cluster %s already completed, skipping...", job.Name())
				skippedMu.Lock()
				skippedCount++
				skippedMu.Unlock()

				return nil
			}

			// Update state to show job started
			br.updateState(job.Name(), StageNew)

			// Execute the job
			if err := job.Execute(ctx, client, config); err != nil {
				return fmt.Errorf("job %s failed: %w", job.Name(), err)
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

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}

	// Batch sleep logic: if fewer than half were skipped, sleep before next batch
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

// updateState updates and persists cluster state (thread-safe)
func (br *BatchRunner) updateState(name string, stage Stage) {
	br.stateMu.Lock()
	defer br.stateMu.Unlock()

	cs := br.statuses[name]
	if cs == nil {
		return
	}

	cs.Stage = stage

	switch stage {
	case StageNew:
		cs.New = true
	case StageInfra:
		cs.Infra = true
	case StageCreated:
		cs.Created = true
	case StageImported:
		cs.Imported = true
	case StageProvisioned:
		cs.Provisioned = true
	case StageRegistered:
		cs.Registered = true
	}

	if err := SaveClusterState(br.statePath, br.statuses); err != nil {
		logrus.Errorf("failed to save state for %s:%s: %v", name, stage, err)
	}
}
