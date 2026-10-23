package subcommands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rancher/dartboard/internal/actions"
	"github.com/rancher/dartboard/internal/dart"
	"github.com/rancher/dartboard/internal/tofu"
)

// verifies that the cluster state is correctly initialized when only the upstream cluster is deployed.
func TestUpstreamOnlyDeploymentInitializesApplicationState(t *testing.T) {
	r := &dart.Dart{TofuWorkspaceStatePath: t.TempDir()}

	clusters := map[string]tofu.Cluster{"upstream": {Name: "upstream"}}
	if err := importDownstreamClusters(r, clusters, nil, nil); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(r.TofuWorkspaceStatePath, actions.ClustersStateFile)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cluster state not initialized: %v", err)
	}

	statuses, err := actions.LoadClusterState(path)
	if err != nil || statuses == nil || len(statuses) != 0 {
		t.Fatalf("expected empty cluster state, got %v, %v", statuses, err)
	}
}
