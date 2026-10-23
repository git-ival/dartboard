package actions

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveClusterStateAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), ClustersStateFile)

	oldData := []byte("old checkpoint\n")
	if err := os.WriteFile(path, oldData, 0o644); err != nil {
		t.Fatal(err)
	}

	oldFile, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer oldFile.Close()

	want := map[string]*ClusterStatus{"cluster": {Name: "cluster", Imported: true, Stage: StageImported}}
	if err := SaveClusterState(path, want); err != nil {
		t.Fatal(err)
	}

	got, err := LoadClusterState(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded state = %v, %v; want %v", got, err, want)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state permissions = %o; want 600", info.Mode().Perm())
	}

	unchanged, err := io.ReadAll(oldFile)
	if err != nil || string(unchanged) != string(oldData) {
		t.Fatalf("old checkpoint was modified: %q, %v", unchanged, err)
	}

	assertNoStateTemps(t, filepath.Dir(path))
}

func TestSaveClusterStateFailureCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, ClustersStateFile)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(path, "checkpoint")
	if err := os.WriteFile(marker, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := SaveClusterState(path, map[string]*ClusterStatus{}); err == nil {
		t.Fatal("expected rename failure")
	}

	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "preserve" {
		t.Fatalf("previous checkpoint lost: %q, %v", data, err)
	}

	assertNoStateTemps(t, dir)

	if err := SaveClusterState(filepath.Join(marker, ClustersStateFile), nil); err == nil {
		t.Fatal("expected directory creation failure")
	}
}

func assertNoStateTemps(t *testing.T, dir string) {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(dir, ".clusters-state-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary state files remain: %v, %v", matches, err)
	}
}
