package actions

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rancher/dartboard/internal/dart"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type fakeClusterResource struct {
	cluster   *provv1.Cluster
	getErr    error
	createErr error
	creates   int
}

func (resource *fakeClusterResource) Get(context.Context, string, metav1.GetOptions) (*provv1.Cluster, error) {
	if resource.getErr != nil {
		return nil, resource.getErr
	}

	if resource.cluster == nil {
		return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "clusters"}, "test")
	}

	return resource.cluster.DeepCopy(), nil
}

func (resource *fakeClusterResource) Create(_ context.Context, cluster *provv1.Cluster, _ metav1.CreateOptions) (*provv1.Cluster, error) {
	resource.creates++
	resource.cluster = cluster.DeepCopy()

	resource.cluster.UID = "remote-identity"
	if resource.createErr != nil {
		return nil, resource.createErr
	}

	return resource.cluster.DeepCopy(), nil
}

func TestEnsureClusterResourceResumesWithoutDuplicateCreation(t *testing.T) {
	for _, scenario := range []struct {
		createErr error
		name      string
	}{
		{name: "normal redeploy"},
		{name: "response lost after server creation", createErr: errors.New("connection lost")},
		{name: "already exists race", createErr: apierrors.NewAlreadyExists(schema.GroupResource{Resource: "clusters"}, "test")},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			resource := &fakeClusterResource{createErr: scenario.createErr}
			desired := &provv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: fleetNamespace}}

			_, firstErr := ensureClusterResource(context.Background(), resource, desired)
			if scenario.createErr != nil && !apierrors.IsAlreadyExists(scenario.createErr) && !errors.Is(firstErr, scenario.createErr) {
				t.Fatalf("expected original creation error, got %v", firstErr)
			}

			cluster, err := ensureClusterResource(context.Background(), resource, desired)
			if err != nil {
				t.Fatal(err)
			}

			if resource.creates != 1 || cluster.UID != "remote-identity" {
				t.Fatalf("resource recreated on resume: creates=%d, UID=%s", resource.creates, cluster.UID)
			}
		})
	}
}

func TestEnsureClusterResourceDoesNotCreateOnLookupFailure(t *testing.T) {
	failure := errors.New("access denied")
	resource := &fakeClusterResource{getErr: failure}

	_, err := ensureClusterResource(context.Background(), resource, &provv1.Cluster{})
	if !errors.Is(err, failure) || resource.creates != 0 {
		t.Fatalf("lookup error triggered creation: creates=%d, err=%v", resource.creates, err)
	}
}

func TestHarvesterProvisioningMachineConfig(t *testing.T) {
	recipe := &dart.Dart{TofuVariables: map[string]any{
		"network":  map[string]any{"name": "network", "namespace": "harvester-public"},
		"ssh_user": "opensuse",
	}}
	template := dart.ClusterTemplate{
		NodeConfig: &dart.NodeConfig{Harvester: &dart.HarvesterNodeConfig{
			CPU: 2, Memory: 8, Password: "test", ImageName: "image", ImageNamespace: "images", Namespace: "vms",
			Disks: []dart.HarvesterDisk{{Size: 35, Bus: "virtio"}},
		}},
		ClusterConfig: &dart.ClusterConfig{
			Provider: dart.HarvesterProvider,
			MachinePools: []dart.MachinePools{{MachinePoolConfig: dart.MachinePoolConfig{
				Quantity: 3, ControlPlane: true, Etcd: true, Worker: true,
			}}},
		},
	}

	configs, err := provisioningMachineConfigs(recipe, template)
	if err != nil {
		t.Fatal(err)
	}

	pool := configs.HarvesterMachineConfigs.HarvesterMachineConfig[0]
	if pool.NetworkName != "harvester-public/network" || pool.SSHUser != "opensuse" || pool.ImageName != "images/image" || len(pool.Roles.Roles) != 3 {
		t.Fatalf("incorrect provider configuration: %+v", pool)
	}

	converted := ConvertConfigToClusterConfig(template.ClusterConfig)
	if len(converted.MachinePools) != 1 || converted.MachinePools[0].MachinePoolConfig.Quantity != 3 {
		t.Fatalf("lost machine pool configuration: %+v", converted)
	}

	template.ClusterConfig.MachinePools[0].MachinePoolConfig.Quantity = 0
	if _, err := provisioningMachineConfigs(recipe, template); err == nil {
		t.Fatal("expected invalid quantity to be rejected")
	}
}

func TestRegistrationPacingSkipsReadyClusters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		pacer, err := newClusterPacer(time.Minute, 0)
		if err != nil {
			t.Fatal(err)
		}

		if err := pacer.wait(context.Background()); err != nil {
			t.Fatal(err)
		}

		cluster := &provv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test"}}
		cluster.Status.Ready = true
		resource := &fakeClusterResource{cluster: cluster}
		start := time.Now()

		if _, err := prepareClusterResource(context.Background(), resource, cluster, pacer); err != nil {
			t.Fatal(err)
		}

		if time.Now() != start || resource.creates != 0 {
			t.Fatal("ready cluster was delayed or recreated")
		}
	})
}

func TestCreationPacingOnlyWaitsForMissingClusters(t *testing.T) {
	for _, exists := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			pacer, err := newClusterPacer(time.Minute, 0)
			if err != nil {
				t.Fatal(err)
			}

			if err := pacer.wait(context.Background()); err != nil {
				t.Fatal(err)
			}

			cluster := &provv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test"}}

			resource := &fakeClusterResource{}
			if exists {
				resource.cluster = cluster
			}

			start := time.Now()

			if _, err := prepareClusterResource(context.Background(), resource, cluster, pacer); err != nil {
				t.Fatal(err)
			}

			expected := time.Minute
			if exists {
				expected = 0
			}

			if elapsed := time.Since(start); elapsed != expected {
				t.Fatalf("exists=%t: expected %s admission wait, got %s", exists, expected, elapsed)
			}
		})
	}
}
