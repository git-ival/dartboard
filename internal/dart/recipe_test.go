package dart

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseClusterPacing(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		yaml     string
		interval time.Duration
		jitter   float64
	}{
		{name: "defaults", yaml: "cluster_batch_size: 10\n", interval: 5 * time.Second, jitter: 0.2},
		{name: "overrides", yaml: "cluster_start_interval: 30s\ncluster_start_jitter: 0.5\n", interval: 30 * time.Second, jitter: 0.5},
		{name: "disabled", yaml: "cluster_start_interval: 0s\ncluster_start_jitter: 0\n"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "dart.yaml")
			if err := os.WriteFile(path, []byte(scenario.yaml), 0o600); err != nil {
				t.Fatal(err)
			}

			recipe, err := Parse(path)
			if err != nil {
				t.Fatal(err)
			}

			if recipe.ClusterStartInterval != scenario.interval || recipe.ClusterStartJitter != scenario.jitter {
				t.Fatalf("expected interval=%s jitter=%g, got interval=%s jitter=%g", scenario.interval, scenario.jitter, recipe.ClusterStartInterval, recipe.ClusterStartJitter)
			}
		})
	}
}
