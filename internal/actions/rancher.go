package actions

import (
	"context"
	"fmt"
	"log"

	"github.com/rancher/dartboard/internal/dart"
	"github.com/rancher/dartboard/internal/tofu"
	yaml "gopkg.in/yaml.v2"

	"github.com/rancher/shepherd/clients/rancher"
	management "github.com/rancher/shepherd/clients/rancher/generated/management/v3"
	shepherdclusters "github.com/rancher/shepherd/extensions/clusters"
	shepherdtokens "github.com/rancher/shepherd/extensions/token"
	"github.com/rancher/shepherd/pkg/session"

	"github.com/rancher/tests/actions/pipeline"

	provv1 "github.com/rancher/rancher/pkg/apis/provisioning.cattle.io/v1"
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

func SetupRancherClient(rancherConfig *rancher.Config, bootstrapPassword string, session *session.Session) (*rancher.Client, error) {
	adminUser := &management.User{
		Username: "admin",
		Password: bootstrapPassword,
	}

	fmt.Printf("Rancher Config:\nHost: %s\nAdminPassword: %s\nAdminToken: %s\nInsecure: %t\n", rancherConfig.Host, rancherConfig.AdminPassword, rancherConfig.AdminToken, *rancherConfig.Insecure)

	adminToken, err := shepherdtokens.GenerateUserToken(adminUser, rancherConfig.Host)
	if err != nil {
		return nil, fmt.Errorf("error while creating Admin Token with config %v:\n%v", &rancherConfig, err)
	}

	rancherConfig.AdminToken = adminToken.Token

	client, err := rancher.NewClientForConfig(rancherConfig.AdminToken, rancherConfig, session)
	if err != nil {
		return nil, fmt.Errorf("error while setting up Rancher client with config %v:\n%v", rancherConfig, err)
	}

	err = pipeline.PostRancherInstall(client, rancherConfig.AdminPassword)
	if err != nil {
		return nil, fmt.Errorf("error during post- rancher install: %v", err)
	}

	client, err = rancher.NewClientForConfig(rancherConfig.AdminToken, rancherConfig, session)
	if err != nil {
		return nil, fmt.Errorf("error during post- rancher install on re-login: %v", err)
	}

	return client, err
}

func ProvisionDownstreamClusters(r *dart.Dart, templates []dart.ClusterTemplate, rancherClient *rancher.Client) error {
	if r.ClusterBatchSize <= 0 {
		panic("ClusterBatchSize must be > 0")
	}

	for _, template := range r.ClusterTemplates {
		err := ProvisionClustersInBatches(r, template, rancherClient)
		if err != nil {
			return err
		}
	}

	return nil
}

// ProvisionClustersInBatches provisions clusters in batches for better resource management.
// r.ClusterBatchSize determines the maximum concurrent clusters per batch.
func ProvisionClustersInBatches(r *dart.Dart, template dart.ClusterTemplate, rancherClient *rancher.Client) error {
	clusterStatePath := fmt.Sprintf("%s/%s", r.TofuWorkspaceStatePath, ClustersStateFile)

	statuses, err := LoadClusterState(clusterStatePath)
	if err != nil {
		return err
	}

	ctx := context.Background()
	batchNum := 0

	for i := 0; i < template.ClusterCount; i += r.ClusterBatchSize {
		end := min(i+r.ClusterBatchSize, template.ClusterCount)

		// Build jobs for this batch
		jobs := make([]ClusterJob, 0, end-i)
		for k := i; k < end; k++ {
			templateCopy := template
			templateCopy.SetGeneratedName(fmt.Sprintf("%d-%d", batchNum, k-i))
			jobs = append(jobs, ProvisionJob{Template: templateCopy})
		}

		// Run batch
		runner := NewBatchRunner(clusterStatePath, statuses)
		if err := runner.Run(ctx, jobs, rancherClient, nil); err != nil {
			return err
		}

		batchNum++
	}

	return nil
}

func ImportDownstreamClusters(r *dart.Dart, clusters []tofu.Cluster, rancherClient *rancher.Client, rancherConfig *rancher.Config) error {
	if r.ClusterBatchSize <= 0 {
		panic("ClusterBatchSize must be > 0")
	}

	if len(clusters) == 0 {
		fmt.Printf("No importable Clusters were provided.\n")
	}

	err := ImportClustersInBatches(r, clusters, rancherClient, rancherConfig)
	if err != nil {
		return err
	}

	return nil
}

// ImportClustersInBatches imports clusters in batches for better resource management.
func ImportClustersInBatches(r *dart.Dart, clusters []tofu.Cluster, rancherClient *rancher.Client, rancherConfig *rancher.Config) error {
	clusterStatePath := fmt.Sprintf("%s/%s", r.TofuWorkspaceStatePath, ClustersStateFile)

	statuses, err := LoadClusterState(clusterStatePath)
	if err != nil {
		return err
	}

	ctx := context.Background()

	for i := 0; i < len(clusters); i += r.ClusterBatchSize {
		end := min(i+r.ClusterBatchSize, len(clusters))

		// Build jobs for this batch
		jobs := make([]ClusterJob, 0, end-i)
		for _, cluster := range clusters[i:end] {
			jobs = append(jobs, ImportJob{Cluster: cluster})
		}

		// Run batch
		runner := NewBatchRunner(clusterStatePath, statuses)
		if err := runner.Run(ctx, jobs, rancherClient, rancherConfig); err != nil {
			return err
		}
	}

	return nil
}

// createAndWaitForCluster creates a cluster and waits for it to be ready
func createAndWaitForCluster(rancherClient *rancher.Client, rancherConfig *rancher.Config, importCluster *provv1.Cluster) (*provv1.Cluster, error) {
	if _, err := CreateK3SRKE2Cluster(rancherClient, rancherConfig, importCluster); err != nil {
		return nil, fmt.Errorf("error while creating Steve Cluster with Name %s:\n%w", importCluster.Name, err)
	}

	err := BackoffWait(30, func() (finished bool, err error) {
		updatedCluster, _, err := shepherdclusters.GetProvisioningClusterByName(rancherClient, importCluster.Name, importCluster.Namespace)
		if err != nil {
			return false, fmt.Errorf("error while getting Cluster by Name %s in Namespace %s:\n%w", importCluster.Name, importCluster.Namespace, err)
		}

		if updatedCluster.Status.ClusterName != "" {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	updatedCluster, _, err := shepherdclusters.GetProvisioningClusterByName(rancherClient, importCluster.Name, importCluster.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error while getting Cluster by Name %s in Namespace %s:\n%w", importCluster.Name, importCluster.Namespace, err)
	}

	return updatedCluster, nil
}

// performClusterImport imports an external cluster into Rancher
func performClusterImport(rancherClient *rancher.Client, cluster tofu.Cluster, importCluster *provv1.Cluster) (*provv1.Cluster, error) {
	restConfig, err := GetRESTConfigFromPath(cluster.Kubeconfig)
	if err != nil {
		return nil, err
	}
	// Apply client-side rate limiting
	restConfig.QPS = 50
	restConfig.Burst = 100

	updatedCluster, _, err := shepherdclusters.GetProvisioningClusterByName(rancherClient, importCluster.Name, importCluster.Namespace)
	if err != nil {
		return nil, fmt.Errorf("error while getting Cluster by Name %s in Namespace %s:\n%w", importCluster.Name, importCluster.Namespace, err)
	}

	fmt.Printf("Importing Cluster, ID:%s Name:%s\n", updatedCluster.Status.ClusterName, updatedCluster.Name)

	err = shepherdclusters.ImportCluster(rancherClient, updatedCluster, restConfig)
	if err != nil {
		return nil, fmt.Errorf("error while creating Job for importing Cluster %s:\n%w", updatedCluster.Name, err)
	}

	err = BackoffWait(100, func() (finished bool, err error) {
		updatedCluster, _, err = shepherdclusters.GetProvisioningClusterByName(rancherClient, importCluster.Name, importCluster.Namespace)
		if err != nil {
			return false, fmt.Errorf("error while getting Cluster by Name %s in Namespace %s:\n%w", importCluster.Name, importCluster.Namespace, err)
		}

		return updatedCluster.Status.Ready, nil
	})
	if err != nil {
		return nil, err
	}

	return updatedCluster, nil
}

func RegisterCustomClusters(r *dart.Dart, templates []tofu.CustomCluster,
	rancherClient *rancher.Client, rancherConfig *rancher.Config,
) error {
	if r.ClusterBatchSize <= 0 {
		panic("ClusterBatchSize must be > 0")
	}

	for _, template := range templates {
		yamlData, err := yaml.Marshal(template)
		if err != nil {
			log.Fatalf("Error marshaling YAML: %v", err)
		}

		fmt.Printf("\ntofu.CustomCluster:\n%s\n", string(yamlData))
	}

	for _, template := range templates {
		err := RegisterCustomClustersInBatches(r, template, rancherClient, rancherConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

// RegisterCustomClustersInBatches registers custom clusters in batches for better resource management.
func RegisterCustomClustersInBatches(r *dart.Dart, template tofu.CustomCluster, rancherClient *rancher.Client, rancherConfig *rancher.Config) error {
	clusterStatePath := fmt.Sprintf("%s/%s", r.TofuWorkspaceStatePath, ClustersStateFile)

	statuses, err := LoadClusterState(clusterStatePath)
	if err != nil {
		return err
	}

	// Build all custom clusters from template
	customClusters := make([]tofu.CustomCluster, 0, template.ClusterCount)
	nodeBatchSize := len(template.Nodes) / template.ClusterCount

	for i := range template.ClusterCount {
		customCluster := template
		customCluster.Name = fmt.Sprintf("%s-%d", template.Name, i)
		customCluster.Nodes = template.Nodes[i*nodeBatchSize : (i+1)*nodeBatchSize]
		customClusters = append(customClusters, customCluster)
	}

	ctx := context.Background()

	for i := 0; i < len(customClusters); i += r.ClusterBatchSize {
		end := min(i+r.ClusterBatchSize, len(customClusters))

		// Build jobs for this batch
		jobs := make([]ClusterJob, 0, end-i)
		for _, cluster := range customClusters[i:end] {
			jobs = append(jobs, RegisterJob{Template: cluster})
		}

		// Run batch
		runner := NewBatchRunner(clusterStatePath, statuses)
		if err := runner.Run(ctx, jobs, rancherClient, rancherConfig); err != nil {
			return err
		}
	}

	return nil
}

// createMachinePools creates machine pools from template configuration
func createMachinePools(template tofu.CustomCluster) []provv1.RKEMachinePool {
	var machinePools []provv1.RKEMachinePool

	for _, pool := range template.MachinePools {
		newPool := provv1.RKEMachinePool{
			EtcdRole:         pool.Etcd,
			ControlPlaneRole: pool.ControlPlane,
			WorkerRole:       pool.Worker,
			Quantity:         &pool.Quantity,
		}
		machinePools = append(machinePools, newPool)
	}

	return machinePools
}
