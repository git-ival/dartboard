package actions

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rancher/dartboard/internal/dart"
	"github.com/rancher/dartboard/internal/tofu"
	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	shepherdclusters "github.com/rancher/shepherd/extensions/clusters"
	shepherddefaults "github.com/rancher/shepherd/extensions/defaults"
	shepherdtokens "github.com/rancher/shepherd/extensions/token"
	"github.com/rancher/shepherd/pkg/session"
	"github.com/rancher/tests/actions/pipeline"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	kwait "k8s.io/apimachinery/pkg/util/wait"
)

const fleetNamespace = "fleet-default"

func NewRancherConfig(host, adminToken, adminPassword string, insecure bool) rancher.Config {
	defaultBool := false

	return rancher.Config{
		Host:          host,
		AdminToken:    adminToken,
		AdminPassword: adminPassword,
		Insecure:      &insecure,
		Cleanup:       &defaultBool,
	}
}

func SetupRancherClient(rancherConfig *rancher.Config, bootstrapPassword string, rancherSession *session.Session) (*rancher.Client, error) {
	adminUser := &management.User{Username: "admin", Password: bootstrapPassword}

	adminToken, err := shepherdtokens.GenerateUserToken(adminUser, rancherConfig.Host)
	if err != nil {
		return nil, fmt.Errorf("creating admin token: %w", err)
	}

	rancherConfig.AdminToken = adminToken.Token

	client, err := rancher.NewClientForConfig(rancherConfig.AdminToken, rancherConfig, rancherSession)
	if err != nil {
		return nil, fmt.Errorf("setting up Rancher client: %w", err)
	}

	if err := pipeline.PostRancherInstall(client, rancherConfig.AdminPassword); err != nil {
		return nil, fmt.Errorf("post-Rancher install: %w", err)
	}

	return rancher.NewClientForConfig(rancherConfig.AdminToken, rancherConfig, rancherSession)
}

func IsFatalClusterError(err error) bool {
	return errors.Is(err, errCheckpointFailed)
}

func clusterJobClient(client *rancher.Client) (*rancher.Client, error) {
	jobSession := session.NewSession()
	jobSession.CleanupEnabled = false
	config := *client.RancherConfig
	cleanup := false
	config.Cleanup = &cleanup

	return rancher.NewClientForConfig(config.AdminToken, &config, jobSession)
}

func runWithClusterState[Job any](ctx context.Context, recipe *dart.Dart, jobs []Job,
	client *rancher.Client, name func(Job) string,
	run func(context.Context, Job, *clusterStateStore, *rancher.Client, *clusterPacer) error,
) error {
	if len(jobs) == 0 {
		return nil
	}

	pacer, err := newClusterPacer(recipe.ClusterStartInterval, recipe.ClusterStartJitter)
	if err != nil {
		return err
	}

	store, err := openClusterStateStore(filepath.Join(recipe.TofuWorkspaceStatePath, ClustersStateFile))
	if err != nil {
		return fmt.Errorf("%w: %w", errCheckpointFailed, err)
	}

	return runClusterJobs(ctx, jobs, recipe.ClusterBatchSize, name, func(ctx context.Context, job Job) error {
		jobClient, err := clusterJobClient(client)
		if err != nil {
			return err
		}

		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()

		return run(jobCtx, job, store, jobClient, pacer)
	})
}

func ProvisionDownstreamClusters(recipe *dart.Dart, templates []dart.ClusterTemplate, client *rancher.Client) error {
	return ProvisionDownstreamClustersWithContext(context.Background(), recipe, templates, client)
}

func ProvisionDownstreamClustersWithContext(ctx context.Context, recipe *dart.Dart, templates []dart.ClusterTemplate, client *rancher.Client) error {
	var (
		jobs           []dart.ClusterTemplate
		templateErrors []error
	)

	for _, template := range templates {
		if template.ClusterCount < 0 || template.ClusterConfig == nil {
			templateErrors = append(templateErrors, fmt.Errorf("invalid provisioning template %q", template.NamePrefix))
			continue
		}

		for index := range template.ClusterCount {
			job := template
			job.SetGeneratedName(fmt.Sprintf("%d", index))
			jobs = append(jobs, job)
		}
	}

	err := runWithClusterState(ctx, recipe, jobs, client,
		func(job dart.ClusterTemplate) string { return job.GeneratedName() },
		func(ctx context.Context, job dart.ClusterTemplate, store *clusterStateStore, client *rancher.Client, pacer *clusterPacer) error {
			return reconcileProvisionedCluster(ctx, recipe, job, store, client, pacer)
		})

	return errors.Join(errors.Join(templateErrors...), err)
}

func ProvisionClustersInBatches(recipe *dart.Dart, template dart.ClusterTemplate, client *rancher.Client) error {
	return ProvisionDownstreamClusters(recipe, []dart.ClusterTemplate{template}, client)
}

func reconcileProvisionedCluster(ctx context.Context, recipe *dart.Dart, template dart.ClusterTemplate, store *clusterStateStore, client *rancher.Client, pacer *clusterPacer) error {
	name := template.GeneratedName()
	if err := validateClusterName(name); err != nil {
		return err
	}

	legacyPattern := regexp.MustCompile("^" + regexp.QuoteMeta(template.NamePrefix) + `-\d+-\d+$`)

	store.mu.Lock()

	var legacyName string

	for key, status := range store.statuses {
		if legacyPattern.MatchString(key) && status != nil {
			legacyName = key
			break
		}
	}
	store.mu.Unlock()

	if legacyName != "" {
		return fmt.Errorf("legacy checkpoint %q has no remote resource identity; reconcile its random-named Rancher cluster before migrating to %q", legacyName, name)
	}

	cluster, err := lookupCluster(ctx, client, name)
	if err != nil {
		return err
	}

	if cluster == nil {
		if err := pacer.wait(ctx); err != nil {
			return err
		}

		if err := store.advance(name, StageNew); err != nil {
			return err
		}

		cluster, err = createNamedProvisioningCluster(ctx, recipe, client, template)
		if err != nil {
			return err
		}
	}

	if err := store.checkpoint(name, StageCreated, string(cluster.UID)); err != nil {
		return err
	}

	if _, err := waitForCluster(ctx, client, name, func(cluster *provv1.Cluster) bool { return cluster.Status.Ready }); err != nil {
		return err
	}

	return store.advance(name, StageProvisioned)
}

func ImportDownstreamClusters(recipe *dart.Dart, clusters []tofu.Cluster, client *rancher.Client, config *rancher.Config) error {
	return ImportDownstreamClustersWithContext(context.Background(), recipe, clusters, client, config)
}

func ImportDownstreamClustersWithContext(ctx context.Context, recipe *dart.Dart, clusters []tofu.Cluster, client *rancher.Client, _ *rancher.Config) error {
	return runWithClusterState(ctx, recipe, clusters, client,
		func(cluster tofu.Cluster) string { return cluster.Name }, reconcileImportedCluster)
}

func ImportClustersInBatches(recipe *dart.Dart, clusters []tofu.Cluster, client *rancher.Client, config *rancher.Config) error {
	return ImportDownstreamClusters(recipe, clusters, client, config)
}

func reconcileImportedCluster(ctx context.Context, target tofu.Cluster, store *clusterStateStore, client *rancher.Client, pacer *clusterPacer) error {
	cluster, err := prepareClusterRegistration(ctx, client, &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: target.Name, Namespace: fleetNamespace},
	}, pacer)
	if err != nil {
		return err
	}

	if err := store.checkpoint(target.Name, StageCreated, string(cluster.UID)); err != nil {
		return err
	}

	if !cluster.Status.Ready {
		cluster, err = waitForCluster(ctx, client, target.Name, func(cluster *provv1.Cluster) bool { return cluster.Status.ClusterName != "" })
		if err != nil {
			return err
		}

		if err := pacer.wait(ctx); err != nil {
			return err
		}

		cluster, err = waitForCluster(ctx, client, target.Name, func(cluster *provv1.Cluster) bool { return cluster.Status.ClusterName != "" })
		if err != nil {
			return err
		}

		if !cluster.Status.Ready {
			if err := applyClusterImport(ctx, client, target.Kubeconfig, cluster.Status.ClusterName); err != nil {
				return err
			}
		}

		cluster, err = waitForCluster(ctx, client, target.Name, func(cluster *provv1.Cluster) bool { return cluster.Status.Ready })
		if err != nil {
			return err
		}
	}

	if podErrors := StatusPodsWithTimeout(client, cluster.Status.ClusterName, shepherddefaults.OneMinuteTimeout); len(podErrors) != 0 {
		return fmt.Errorf("checking pods: %w", errors.Join(podErrors...))
	}

	return store.advance(target.Name, StageImported)
}

func RegisterCustomClusters(recipe *dart.Dart, templates []tofu.CustomCluster, client *rancher.Client, config *rancher.Config) error {
	return RegisterCustomClustersWithContext(context.Background(), recipe, templates, client, config)
}

func RegisterCustomClustersWithContext(ctx context.Context, recipe *dart.Dart, templates []tofu.CustomCluster, client *rancher.Client, _ *rancher.Config) error {
	var (
		jobs           []tofu.CustomCluster
		templateErrors []error
	)

	for _, template := range templates {
		if template.ClusterCount <= 0 || len(template.Nodes)%template.ClusterCount != 0 {
			templateErrors = append(templateErrors, fmt.Errorf("custom template %q needs a positive cluster count and evenly partitioned nodes", template.Name))
			continue
		}

		nodesPerCluster := len(template.Nodes) / template.ClusterCount
		for index := range template.ClusterCount {
			job := template
			job.Name = fmt.Sprintf("%s-%d", template.Name, index)
			job.Nodes = template.Nodes[index*nodesPerCluster : (index+1)*nodesPerCluster]
			jobs = append(jobs, job)
		}
	}

	err := runWithClusterState(ctx, recipe, jobs, client,
		func(job tofu.CustomCluster) string { return job.Name }, reconcileCustomCluster)

	return errors.Join(errors.Join(templateErrors...), err)
}

func RegisterCustomClustersInBatches(recipe *dart.Dart, template tofu.CustomCluster, client *rancher.Client, config *rancher.Config) error {
	return RegisterCustomClusters(recipe, []tofu.CustomCluster{template}, client, config)
}

func createMachinePools(template tofu.CustomCluster) []provv1.RKEMachinePool {
	var pools []provv1.RKEMachinePool
	for _, pool := range template.MachinePools {
		pools = append(pools, provv1.RKEMachinePool{
			EtcdRole: pool.Etcd, ControlPlaneRole: pool.ControlPlane,
			WorkerRole: pool.Worker, Quantity: &pool.Quantity,
		})
	}

	return pools
}

func reconcileCustomCluster(ctx context.Context, template tofu.CustomCluster, store *clusterStateStore, client *rancher.Client, pacer *clusterPacer) error {
	desired := &provv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: template.Name, Namespace: fleetNamespace},
		Spec: provv1.ClusterSpec{
			KubernetesVersion: template.DistroVersion,
			DefaultPodSecurityAdmissionConfigurationTemplateName: psactRancherPrivileged,
			RKEConfig: &provv1.RKEConfig{},
		},
	}

	cluster, err := prepareClusterRegistration(ctx, client, desired, pacer)
	if err != nil {
		return err
	}

	if err := store.checkpoint(template.Name, StageCreated, string(cluster.UID)); err != nil {
		return err
	}

	if !cluster.Status.Ready {
		if _, err := waitForCluster(ctx, client, template.Name, func(cluster *provv1.Cluster) bool { return cluster.Status.ClusterName != "" }); err != nil {
			return err
		}

		desired.UID = cluster.UID
		desired.Spec.RKEConfig.MachinePools = createMachinePools(template)

		if err := pacer.wait(ctx); err != nil {
			return err
		}

		if _, err := registerCustomClusterWithContext(ctx, client, desired, template.Nodes); err != nil {
			return err
		}
	}

	if store.snapshot(template.Name).Registered && cluster.Status.Ready {
		logrus.Infof("Cluster %s is already registered and ready", template.Name)
		return nil
	}

	clusterObject, err := client.Steve.SteveType(shepherdclusters.ProvisioningSteveResourceType).ByID(fleetNamespace + "/" + template.Name)
	if err != nil {
		return err
	}

	if err := VerifyCluster(client, client.RancherConfig, clusterObject); err != nil {
		return err
	}

	return store.advance(template.Name, StageRegistered)
}

func validateClusterName(name string) error {
	if problems := validation.IsDNS1123Label(name); len(problems) != 0 {
		return fmt.Errorf("invalid cluster name %q: %s", name, strings.Join(problems, "; "))
	}

	return nil
}

func lookupCluster(ctx context.Context, client *rancher.Client, name string) (*provv1.Cluster, error) {
	if err := validateClusterName(name); err != nil {
		return nil, err
	}

	provisioningClient, err := client.GetKubeAPIProvisioningClient()
	if err != nil {
		return nil, err
	}

	cluster, err := provisioningClient.Clusters(fleetNamespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}

	return cluster, err
}

// ensureCluster ensures that the desired cluster exists in the provisioning API.
// It validates the cluster name and either retrieves the existing cluster or creates a new one.
func ensureCluster(ctx context.Context, client *rancher.Client, desired *provv1.Cluster) (*provv1.Cluster, error) {
	if err := validateClusterName(desired.Name); err != nil {
		return nil, err
	}

	provisioningClient, err := client.GetKubeAPIProvisioningClient()
	if err != nil {
		return nil, err
	}

	return ensureClusterResource(ctx, provisioningClient.Clusters(fleetNamespace), desired)
}

type clusterResource interface {
	Get(context.Context, string, metav1.GetOptions) (*provv1.Cluster, error)
	Create(context.Context, *provv1.Cluster, metav1.CreateOptions) (*provv1.Cluster, error)
}

func prepareClusterRegistration(ctx context.Context, client *rancher.Client, desired *provv1.Cluster, pacer *clusterPacer) (*provv1.Cluster, error) {
	if err := validateClusterName(desired.Name); err != nil {
		return nil, err
	}

	provisioningClient, err := client.GetKubeAPIProvisioningClient()
	if err != nil {
		return nil, err
	}

	return prepareClusterResource(ctx, provisioningClient.Clusters(fleetNamespace), desired, pacer)
}

func prepareClusterResource(ctx context.Context, resource clusterResource, desired *provv1.Cluster, pacer *clusterPacer) (*provv1.Cluster, error) {
	cluster, err := resource.Get(ctx, desired.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	if err == nil {
		return cluster, nil
	}

	if err := pacer.wait(ctx); err != nil {
		return nil, err
	}

	return ensureClusterResource(ctx, resource, desired)
}

// ensureClusterResource ensures that the desired cluster resource exists.
// It attempts to retrieve the cluster and creates it if it does not exist.
func ensureClusterResource(ctx context.Context, resource clusterResource, desired *provv1.Cluster) (*provv1.Cluster, error) {
	cluster, err := resource.Get(ctx, desired.Name, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		return cluster, err
	}

	cluster, err = resource.Create(ctx, desired, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return resource.Get(ctx, desired.Name, metav1.GetOptions{})
	}

	return cluster, err
}

func waitForCluster(ctx context.Context, client *rancher.Client, name string, ready func(*provv1.Cluster) bool) (*provv1.Cluster, error) {
	var cluster *provv1.Cluster

	err := kwait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		var err error

		cluster, err = lookupCluster(ctx, client, name)

		return cluster != nil && ready(cluster), err
	})

	return cluster, err
}
