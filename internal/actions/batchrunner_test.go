package actions

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rancher/dartboard/internal/tofu"
)

func TestCollectBatchResultsAggregatesFailures(t *testing.T) {
	results := make(chan jobResult, 3)
	results <- jobResult{err: errors.New("first")}

	results <- jobResult{skipped: true}

	results <- jobResult{err: errors.New("second")}

	skipped, err := collectBatchResults(results, 3)
	if skipped != 1 {
		t.Fatalf("expected one skipped job, got %d", skipped)
	}

	if err == nil || !errors.Is(err, errBatchFailed) {
		t.Fatalf("expected aggregated batch failure, got %v", err)
	}
}

func TestBatchRunnerReturnsPersistenceFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), ClustersStateFile)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}

	statuses := map[string]*ClusterStatus{
		"first":  {Name: "first", Imported: true},
		"second": {Name: "second", Imported: true},
	}
	batch := []tofu.Cluster{{Name: "first"}, {Name: "second"}}
	runner := NewSequencedBatchRunner[tofu.Cluster](len(batch))
	done := make(chan error, 1)

	go func() {
		done <- runner.Run(batch, statuses, path, nil, nil)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, errBatchFailed) {
			t.Fatalf("expected persistence failure, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("batch runner blocked after persistence failure")
	}

	assertNoStateTemps(t, filepath.Dir(path))
}

func TestBatchWriterDrainsAfterPersistenceFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), ClustersStateFile)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}

	runner := NewSequencedBatchRunner[tofu.Cluster](1)
	statuses := map[string]*ClusterStatus{"cluster": {Name: "cluster"}}

	runner.wgWriter.Add(1)

	go runner.writer(statuses, path)

	done := make(chan error, 1)

	go func() {
		for range 10 {
			runner.Updates <- stateUpdate{Name: "cluster", Stage: StageImported}
		}

		done <- runner.Wait()
	}()

	select {
	case err := <-done:
		if !errors.Is(err, errBatchFailed) {
			t.Fatalf("expected persistence failure from Wait, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writer stopped draining updates after persistence failure")
	}
}

func TestBatchRunnerWaitsForPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), ClustersStateFile)
	statuses := map[string]*ClusterStatus{
		"first":  {Name: "first", Imported: true},
		"second": {Name: "second", Imported: true},
	}
	batch := []tofu.Cluster{{Name: "first"}, {Name: "second"}}

	runner := NewSequencedBatchRunner[tofu.Cluster](len(batch))
	if err := runner.Run(batch, statuses, path, nil, nil); err != nil {
		t.Fatal(err)
	}

	persisted, err := LoadClusterState(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, cluster := range batch {
		status := persisted[cluster.Name]
		if status == nil || !status.New || !status.Imported {
			t.Fatalf("batch returned before persisting %s: %+v", cluster.Name, status)
		}
	}
}
