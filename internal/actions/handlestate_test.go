package actions

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestClusterStateConcurrentCheckpoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), ClustersStateFile)

	store, err := openClusterStateStore(path)
	if err != nil {
		t.Fatal(err)
	}

	var workers sync.WaitGroup
	for index := range 12 {
		workers.Go(func() {
			name := fmt.Sprintf("cluster-%d", index)
			for _, stage := range []Stage{StageNew, StageCreated, StageRegistered, StageNew} {
				if err := store.advance(name, stage); err != nil {
					t.Error(err)
				}

				_ = store.snapshot(name)

				if _, err := LoadClusterState(path); err != nil {
					t.Errorf("reading atomic checkpoint: %v", err)
				}
			}
		})
	}

	workers.Wait()

	statuses, err := LoadClusterState(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(statuses) != 12 {
		t.Fatalf("expected 12 statuses, got %d", len(statuses))
	}

	for name, status := range statuses {
		if status.Stage != StageRegistered || !status.Registered || !status.Created {
			t.Errorf("regressed checkpoint for %s: %+v", name, status)
		}
	}

	copy := store.snapshot("cluster-0")

	copy.Registered = false
	if copy.Registered || !store.snapshot("cluster-0").Registered {
		t.Fatal("snapshot mutation escaped into the store")
	}
}

func TestClusterStateWriteFailurePreservesCheckpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), ClustersStateFile)

	store, err := openClusterStateStore(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.advance("cluster", StageCreated); err != nil {
		t.Fatal(err)
	}

	store.path = t.TempDir()
	if err := store.advance("cluster", StageRegistered); !errors.Is(err, errCheckpointFailed) {
		t.Fatalf("expected checkpoint failure, got %v", err)
	}

	if store.snapshot("cluster").Registered {
		t.Fatal("failed checkpoint was committed in memory")
	}

	statuses, err := LoadClusterState(path)
	if err != nil || statuses["cluster"].Stage != StageCreated {
		t.Fatalf("previous checkpoint damaged: %v, %v", statuses, err)
	}

	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 1 {
		t.Fatalf("unexpected checkpoint artifacts: %v, %v", files, err)
	}
}

func TestClusterStateDoesNotReuseCompletionForRecreatedResource(t *testing.T) {
	store, err := openClusterStateStore(filepath.Join(t.TempDir(), ClustersStateFile))
	if err != nil {
		t.Fatal(err)
	}

	if err := store.checkpoint("cluster", StageCreated, "first-uid"); err != nil {
		t.Fatal(err)
	}

	if err := store.advance("cluster", StageRegistered); err != nil {
		t.Fatal(err)
	}

	if err := store.checkpoint("cluster", StageCreated, "replacement-uid"); err != nil {
		t.Fatal(err)
	}

	status := store.snapshot("cluster")
	if status.Registered || status.Stage != StageCreated || status.ResourceUID != "replacement-uid" {
		t.Fatalf("recreated resource inherited completion: %+v", status)
	}
}
