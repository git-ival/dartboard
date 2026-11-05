package subcommands

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/dartboard/internal/actions"
	"github.com/rancher/dartboard/internal/dart"
	"github.com/rancher/dartboard/internal/tofu"
)

func TestRunDownstreamPhasesAttemptsAllAfterFailures(t *testing.T) {
	first := errors.New("import failed")
	last := errors.New("provision failed")
	attempts := 0

	err := runDownstreamPhases(context.Background(),
		func() error { attempts++; return first },
		func() error { attempts++; return nil },
		func() error { attempts++; return last })
	if attempts != 3 || !errors.Is(err, first) || !errors.Is(err, last) {
		t.Fatalf("expected all phases and both failures: attempts=%d err=%v", attempts, err)
	}
}

func TestRunDownstreamPhasesStopsOnCheckpointFailure(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	recipe := &dart.Dart{TofuWorkspaceStatePath: filepath.Join(file, "state"), ClusterBatchSize: 1}

	err := runDownstreamPhases(context.Background(),
		func() error {
			return actions.ImportDownstreamClusters(recipe, []tofu.Cluster{{Name: "test"}}, nil, nil)
		},
		func() error {
			t.Error("attempted phase after checkpoint failure")
			return nil
		})
	if !actions.IsFatalClusterError(err) {
		t.Fatalf("expected fatal checkpoint failure, got %v", err)
	}
}

func TestRunDownstreamPhasesStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	err := runDownstreamPhases(ctx,
		func() error { cancel(); return nil },
		func() error { t.Error("attempted phase after cancellation"); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
