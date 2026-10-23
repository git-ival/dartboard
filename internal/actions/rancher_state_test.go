package actions

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/dartboard/internal/dart"
)

func TestImportNoDownstreamInitializesStateWithoutReplacingExisting(t *testing.T) {
	r := &dart.Dart{TofuWorkspaceStatePath: t.TempDir()}

	path := filepath.Join(r.TofuWorkspaceStatePath, ClustersStateFile)
	if err := ImportDownstreamClusters(r, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("application state not initialized: %v", err)
	}

	statuses, err := LoadClusterState(path)
	if err != nil || statuses == nil || len(statuses) != 0 {
		t.Fatalf("expected persisted empty state, got %v, %v", statuses, err)
	}

	existing := []byte("# Existing checkpoint must remain unchanged.\ncluster:\n  name: cluster\n  imported: true\n")
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ImportDownstreamClusters(r, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil || string(data) != string(existing) {
		t.Fatalf("existing checkpoint changed: %q, %v", data, err)
	}
}

func TestImportNoDownstreamReturnsStateInitializationFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	r := &dart.Dart{TofuWorkspaceStatePath: path}
	if err := ImportDownstreamClusters(r, nil, nil, nil); err == nil {
		t.Fatal("expected state initialization failure")
	}
}
